// The Mac app: one window on the local server (started by launchd, see
// `kanso init`), and the reminders scheduled in macOS so they arrive even
// when the app is closed. Its name comes from Info.plist (scripts/brand.sh),
// its words from the server's language files (web/locales, keys « mac.* »).
import Cocoa
import ServiceManagement
import UserNotifications
import WebKit

let appName = Bundle.main.object(forInfoDictionaryKey: "CFBundleName") as? String ?? "Kanso"

/// The server's port, from the file `kanso init` writes; 48219 until there is one.
let serverURL: URL = {
    let file = FileManager.default.homeDirectoryForCurrentUser
        .appendingPathComponent("Library/Application Support/\(appName)/config.json")
    var port = 48219
    if let data = try? Data(contentsOf: file),
        let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
        let p = json["port"] as? Int, p > 0
    {
        port = p
    }
    return URL(string: "http://localhost:\(port)/")!
}()
let background = NSColor(srgbRed: 15 / 255, green: 16 / 255, blue: 18 / 255, alpha: 1)

/// A notification the server asks for (GET /api/reminders).
struct Reminder: Decodable {
    let id: String
    let at: String
    let title: String
    let body: String
    let view: String
}

final class AppDelegate: NSObject, NSApplicationDelegate, NSWindowDelegate, WKNavigationDelegate, WKUIDelegate,
    WKScriptMessageHandler, UNUserNotificationCenterDelegate
{
    var window: NSWindow!
    var web: WKWebView!
    var syncTimer: Timer?
    var pendingSync: DispatchWorkItem?
    var texts: [String: Any] = [:]

    func applicationDidFinishLaunching(_ notification: Notification) {
        let config = WKWebViewConfiguration()
        config.userContentController.add(self, name: "inc")
        web = WKWebView(frame: .zero, configuration: config)
        web.navigationDelegate = self
        web.uiDelegate = self
        web.setValue(false, forKey: "drawsBackground")

        window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 1400, height: 880),
            styleMask: [.titled, .closable, .miniaturizable, .resizable], backing: .buffered, defer: false)
        window.title = appName
        window.titleVisibility = .hidden
        window.titlebarAppearsTransparent = true
        window.appearance = NSAppearance(named: .darkAqua)
        window.backgroundColor = background
        window.minSize = NSSize(width: 900, height: 600)
        window.isReleasedWhenClosed = false
        window.delegate = self
        window.contentView = web
        if !window.setFrameUsingName("main"), let screen = NSScreen.main?.visibleFrame {
            window.setContentSize(NSSize(width: min(1440, screen.width - 80), height: min(920, screen.height - 60)))
            window.center()
        }
        window.setFrameAutosaveName("main")
        window.makeKeyAndOrderFront(nil)
        web.load(URLRequest(url: serverURL))

        let center = UNUserNotificationCenter.current()
        center.delegate = self
        center.requestAuthorization(options: [.alert, .sound, .badge]) { granted, error in
            NSLog("%@: notifications %@ %@", appName, granted ? "allowed" : "refused", error?.localizedDescription ?? "")
            DispatchQueue.main.async {
                self.syncReminders()
                self.reportStatus()
            }
        }
        syncTimer = Timer.scheduledTimer(withTimeInterval: 30 * 60, repeats: true) { _ in self.syncReminders() }
        NSApp.activate(ignoringOtherApps: true)
    }

    // Closing the window keeps the app (and its reminders) alive; the Dock icon brings it back.
    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool { false }

    func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows flag: Bool) -> Bool {
        if !flag { window.makeKeyAndOrderFront(nil) }
        return true
    }

    // MARK: reminders

    /// syncReminders replaces every pending notification with the server's list:
    /// a paid bill or a deadline ticked as done stops being announced.
    func syncReminders() {
        var request = URLRequest(url: serverURL.appendingPathComponent("api/reminders"))
        request.timeoutInterval = 15
        // The Mac's languages, in order: the server writes the reminders in the first one it speaks.
        request.setValue(Locale.preferredLanguages.joined(separator: ","), forHTTPHeaderField: "Accept-Language")
        URLSession.shared.dataTask(with: request) { data, _, _ in
            guard let data, let list = try? JSONDecoder().decode([Reminder].self, from: data) else { return }
            let center = UNUserNotificationCenter.current()
            center.removeAllPendingNotificationRequests()
            let parse = DateFormatter()
            parse.locale = Locale(identifier: "en_US_POSIX")
            parse.dateFormat = "yyyy-MM-dd'T'HH:mm"
            var scheduled = 0
            for r in list where scheduled < 60 {  // macOS keeps 64 pending notifications per app
                guard let date = parse.date(from: r.at), date > Date() else { continue }
                let content = UNMutableNotificationContent()
                content.title = r.title
                content.body = r.body
                content.sound = .default
                content.userInfo = ["view": r.view]
                let when = Calendar.current.dateComponents([.year, .month, .day, .hour, .minute], from: date)
                let trigger = UNCalendarNotificationTrigger(dateMatching: when, repeats: false)
                center.add(UNNotificationRequest(identifier: r.id, content: content, trigger: trigger))
                scheduled += 1
            }
            NSLog("%@: %d reminders scheduled", appName, scheduled)
        }.resume()
    }

    /// The page talks to the app: « changed » (its data changed: sync the
    /// reminders once it settles), or what only the app can do, from Settings.
    func userContentController(_ controller: WKUserContentController, didReceive message: WKScriptMessage) {
        guard let body = message.body as? [String: Any], let type = body["type"] as? String else {
            pendingSync?.cancel()
            let work = DispatchWorkItem { self.syncReminders() }
            pendingSync = work
            DispatchQueue.main.asyncAfter(deadline: .now() + 2, execute: work)
            return
        }
        switch type {
        case "test": testReminder()
        case "pickFolder": pickFolder(move: body["move"] as? Bool ?? false)
        case "loginItem": setLoginItem(body["on"] as? Bool ?? false)
        case "openNotifications":
            let url = "x-apple.systempreferences:com.apple.Notifications-Settings.extension?id=\(Bundle.main.bundleIdentifier ?? "")"
            if let u = URL(string: url) { NSWorkspace.shared.open(u) }
        default: break
        }
    }

    // MARK: settings

    /// reportStatus tells the server what only the app knows: notifications
    /// allowed, opening at login. The Settings screen and `kanso doctor` show it.
    func reportStatus() {
        UNUserNotificationCenter.current().getNotificationSettings { settings in
            let state: String
            switch settings.authorizationStatus {
            case .authorized: state = "authorized"
            case .provisional: state = "provisional"
            case .denied: state = "denied"
            default: state = "notDetermined"
            }
            var request = URLRequest(url: serverURL.appendingPathComponent("api/app-status"))
            request.httpMethod = "POST"
            request.setValue("1", forHTTPHeaderField: "X-Inc")
            request.setValue("application/json", forHTTPHeaderField: "Content-Type")
            let version = Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? ""
            request.httpBody = try? JSONSerialization.data(withJSONObject: [
                "notifications": state, "loginItem": SMAppService.mainApp.status == .enabled, "version": version,
            ])
            URLSession.shared.dataTask(with: request) { _, _, _ in
                DispatchQueue.main.async { self.web.evaluateJavaScript("window.kansoAppChanged && window.kansoAppChanged()") }
            }.resume()
        }
    }

    /// setLoginItem opens the app at login, or stops it (macOS may ask to confirm in System Settings).
    func setLoginItem(_ on: Bool) {
        do {
            if on { try SMAppService.mainApp.register() } else { try SMAppService.mainApp.unregister() }
        } catch {
            NSLog("%@: login item: %@", appName, error.localizedDescription)
        }
        reportStatus()
    }

    /// pickFolder lets the user choose a folder, then hands it to the page: to use it, or to move the folder into it.
    func pickFolder(move: Bool) {
        let panel = NSOpenPanel()
        panel.canChooseFiles = false
        panel.canChooseDirectories = true
        panel.canCreateDirectories = true
        panel.allowsMultipleSelection = false
        panel.beginSheetModal(for: window) { result in
            guard result == .OK, let path = panel.url?.path,
                let json = try? JSONEncoder().encode(path), let literal = String(data: json, encoding: .utf8)
            else { return }
            self.web.evaluateJavaScript("window.kansoFolderPicked(\(literal), \(move))")
        }
    }

    func windowDidBecomeKey(_ notification: Notification) { reportStatus() }

    // MARK: words

    /// loadTexts reads the page's language and its file, then names the menus in it.
    func loadTexts() {
        var request = URLRequest(url: serverURL.appendingPathComponent("api/locale"))
        request.setValue(Locale.preferredLanguages.joined(separator: ","), forHTTPHeaderField: "Accept-Language")
        URLSession.shared.dataTask(with: request) { data, _, _ in
            guard let data, let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
                let lang = json["lang"] as? String
            else { return }
            let file = serverURL.appendingPathComponent("locales/\(lang).json")
            URLSession.shared.dataTask(with: file) { data, _, _ in
                guard let data, let dict = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else { return }
                DispatchQueue.main.async {
                    self.texts = dict
                    self.buildMenu()
                }
            }.resume()
        }.resume()
    }

    /// text is the sentence of key (« mac.quit ») with {{name}} as the app's name; the key itself when missing.
    func text(_ key: String) -> String {
        var node: Any? = texts
        for part in key.split(separator: ".") { node = (node as? [String: Any])?[String(part)] }
        return (node as? String ?? key).replacingOccurrences(of: "{{name}}", with: appName)
    }

    func userNotificationCenter(
        _ center: UNUserNotificationCenter, willPresent notification: UNNotification,
        withCompletionHandler done: @escaping (UNNotificationPresentationOptions) -> Void
    ) {
        done([.banner, .sound])
    }

    /// A click on a notification opens the app on the screen it is about.
    func userNotificationCenter(
        _ center: UNUserNotificationCenter, didReceive response: UNNotificationResponse,
        withCompletionHandler done: @escaping () -> Void
    ) {
        let view = (response.notification.request.content.userInfo["view"] as? String ?? "calendar").filter(\.isLetter)
        DispatchQueue.main.async {
            self.window.makeKeyAndOrderFront(nil)
            NSApp.activate(ignoringOtherApps: true)
            self.web.evaluateJavaScript("location.hash = '\(view)'")
        }
        done()
    }

    // MARK: web view

    func isServer(_ url: URL) -> Bool { url.host == serverURL.host && url.port == serverURL.port }

    /// Pages of the app stay in the window; a file of the Inc folder opens in its Mac app
    /// (Aperçu for a PDF), any other link in the default browser.
    func webView(
        _ webView: WKWebView, decidePolicyFor action: WKNavigationAction,
        decisionHandler: @escaping (WKNavigationActionPolicy) -> Void
    ) {
        guard let url = action.request.url, let scheme = url.scheme else { return decisionHandler(.allow) }
        if scheme == "about" || scheme == "data" || (isServer(url) && !url.path.hasPrefix("/files/")) {
            return decisionHandler(.allow)
        }
        decisionHandler(.cancel)
        openOutside(url)
    }

    func webView(
        _ webView: WKWebView, createWebViewWith configuration: WKWebViewConfiguration, for action: WKNavigationAction,
        windowFeatures: WKWindowFeatures
    ) -> WKWebView? {
        if let url = action.request.url { openOutside(url) }
        return nil
    }

    func openOutside(_ url: URL) {
        guard isServer(url), url.path.hasPrefix("/files/") else {
            NSWorkspace.shared.open(url)
            return
        }
        var request = URLRequest(url: serverURL.appendingPathComponent("api/open"))
        request.httpMethod = "POST"
        request.setValue("1", forHTTPHeaderField: "X-Inc")
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try? JSONSerialization.data(withJSONObject: ["path": String(url.path.dropFirst("/files/".count))])
        URLSession.shared.dataTask(with: request).resume()
    }

    /// <input type="file">: the macOS file picker.
    func webView(
        _ webView: WKWebView, runOpenPanelWith parameters: WKOpenPanelParameters, initiatedByFrame frame: WKFrameInfo,
        completionHandler: @escaping ([URL]?) -> Void
    ) {
        let panel = NSOpenPanel()
        panel.allowsMultipleSelection = parameters.allowsMultipleSelection
        panel.canChooseDirectories = false
        panel.beginSheetModal(for: window) { result in completionHandler(result == .OK ? panel.urls : nil) }
    }

    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) { loadTexts() }

    /// The server may still be starting (after a login or an update): try again.
    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) {
        let e = error as NSError
        guard e.domain == NSURLErrorDomain, e.code != NSURLErrorCancelled else { return }
        DispatchQueue.main.asyncAfter(deadline: .now() + 1.5) { webView.load(URLRequest(url: serverURL)) }
    }

    // MARK: menu

    func buildMenu() {
        let main = NSMenu()
        func submenu(_ title: String, _ items: [NSMenuItem]) -> NSMenu {
            let item = NSMenuItem()
            let menu = NSMenu(title: title)
            items.forEach(menu.addItem)
            item.submenu = menu
            main.addItem(item)
            return menu
        }
        func item(_ title: String, _ action: Selector, _ key: String, _ target: AnyObject? = nil) -> NSMenuItem {
            let it = NSMenuItem(title: title, action: action, keyEquivalent: key)
            it.target = target
            return it
        }
        _ = submenu(appName, [
            item(text("mac.settings"), #selector(openSettings), ",", self),
            item(text("mac.test"), #selector(testReminder), "", self),
            .separator(),
            item(text("mac.hide"), #selector(NSApplication.hide(_:)), "h"),
            .separator(),
            item(text("mac.quit"), #selector(NSApplication.terminate(_:)), "q"),
        ])
        _ = submenu(text("mac.edit"), [
            item(text("mac.undo"), Selector(("undo:")), "z"),
            item(text("mac.redo"), Selector(("redo:")), "Z"),
            .separator(),
            item(text("mac.cut"), #selector(NSText.cut(_:)), "x"),
            item(text("mac.copy"), #selector(NSText.copy(_:)), "c"),
            item(text("mac.paste"), #selector(NSText.paste(_:)), "v"),
            item(text("mac.selectAll"), #selector(NSText.selectAll(_:)), "a"),
        ])
        _ = submenu(text("mac.view"), [item(text("mac.reload"), #selector(reload), "r", self)])
        let windows = submenu(text("mac.window"), [
            item(text("mac.minimize"), #selector(NSWindow.performMiniaturize(_:)), "m"),
            item(text("mac.close"), #selector(NSWindow.performClose(_:)), "w"),
        ])
        NSApp.mainMenu = main
        NSApp.windowsMenu = windows
    }

    @objc func reload() { web.reload() }

    @objc func openSettings() {
        window.makeKeyAndOrderFront(nil)
        web.evaluateJavaScript("location.hash = 'settings'")
    }

    /// A notification in 5 seconds: shows the reminders are allowed (System Settings › Notifications otherwise).
    @objc func testReminder() {
        let content = UNMutableNotificationContent()
        content.title = text("mac.testTitle")
        content.body = text("mac.testBody")
        content.sound = .default
        content.userInfo = ["view": "calendar"]
        let trigger = UNTimeIntervalNotificationTrigger(timeInterval: 5, repeats: false)
        UNUserNotificationCenter.current().add(UNNotificationRequest(identifier: "test", content: content, trigger: trigger))
    }
}

let app = NSApplication.shared
let delegate = AppDelegate()
app.delegate = delegate
app.setActivationPolicy(.regular)
app.run()
