// Coddyum Inc.app — the Mac app: one window on the local server (started by
// launchd, see scripts/agent.sh), and the reminders scheduled in macOS so they
// arrive even when the app is closed.
import Cocoa
import UserNotifications
import WebKit

let serverURL = URL(string: "http://localhost:48219/")!
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

    func applicationDidFinishLaunching(_ notification: Notification) {
        buildMenu()
        let config = WKWebViewConfiguration()
        config.userContentController.add(self, name: "inc")
        web = WKWebView(frame: .zero, configuration: config)
        web.navigationDelegate = self
        web.uiDelegate = self
        web.setValue(false, forKey: "drawsBackground")

        window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 1400, height: 880),
            styleMask: [.titled, .closable, .miniaturizable, .resizable], backing: .buffered, defer: false)
        window.title = "Coddyum Inc"
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
            NSLog("Coddyum Inc : notifications %@ %@", granted ? "autorisées" : "refusées", error?.localizedDescription ?? "")
            DispatchQueue.main.async { self.syncReminders() }
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
            NSLog("Coddyum Inc : %d rappels programmés", scheduled)
        }.resume()
    }

    /// The page says its data changed (a payment saved, a deadline ticked): sync once it settles.
    func userContentController(_ controller: WKUserContentController, didReceive message: WKScriptMessage) {
        pendingSync?.cancel()
        let work = DispatchWorkItem { self.syncReminders() }
        pendingSync = work
        DispatchQueue.main.asyncAfter(deadline: .now() + 2, execute: work)
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
        _ = submenu("Coddyum Inc", [
            item("Envoyer un rappel de test", #selector(testReminder), "", self),
            .separator(),
            item("Masquer Coddyum Inc", #selector(NSApplication.hide(_:)), "h"),
            .separator(),
            item("Quitter Coddyum Inc", #selector(NSApplication.terminate(_:)), "q"),
        ])
        _ = submenu("Édition", [
            item("Annuler", Selector(("undo:")), "z"),
            item("Rétablir", Selector(("redo:")), "Z"),
            .separator(),
            item("Couper", #selector(NSText.cut(_:)), "x"),
            item("Copier", #selector(NSText.copy(_:)), "c"),
            item("Coller", #selector(NSText.paste(_:)), "v"),
            item("Tout sélectionner", #selector(NSText.selectAll(_:)), "a"),
        ])
        _ = submenu("Présentation", [item("Recharger", #selector(reload), "r", self)])
        let windows = submenu("Fenêtre", [
            item("Réduire", #selector(NSWindow.performMiniaturize(_:)), "m"),
            item("Fermer", #selector(NSWindow.performClose(_:)), "w"),
        ])
        NSApp.mainMenu = main
        NSApp.windowsMenu = windows
    }

    @objc func reload() { web.reload() }

    /// A notification in 5 seconds: shows the reminders are allowed (Réglages Système › Notifications otherwise).
    @objc func testReminder() {
        let content = UNMutableNotificationContent()
        content.title = "Rappel de test"
        content.body = "Les rappels de Coddyum Inc arrivent bien."
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
