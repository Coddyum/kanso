// pdftext prints the text of each document given on the command line: the
// first pages of a PDF, read by PDFKit; a scanned PDF (no text in it) and a
// picture (a photo of a receipt) are read by Vision, Apple's text
// recognition. Both ship with macOS and run on the Mac: no dependency, nothing sent.
// Each block starts with the record separator (0x1E), then the file path on its own line.
import Foundation
import PDFKit
import Vision

let maxPages = 3
let scanPages = 2  // a scan is read page by page by Vision: slower, so fewer pages
let pictures: Set<String> = ["png", "jpg", "jpeg", "heic", "heif", "tif", "tiff", "gif", "webp", "bmp"]

/// recognize reads the lines of an image, top to bottom; the pieces Vision
/// finds side by side (« TOTAL TTC » … « 4,50 EUR ») are put back on one line.
func recognize(_ image: CGImage) -> String {
    let request = VNRecognizeTextRequest()
    request.recognitionLevel = .accurate
    request.usesLanguageCorrection = true
    if #available(macOS 13.0, *) { request.automaticallyDetectsLanguage = true }
    let wanted = ["fr-FR", "en-US", "es-ES", "de-DE", "it-IT", "pt-BR", "ja-JP", "zh-Hans", "ar-SA"]
    if let supported = try? request.supportedRecognitionLanguages() {
        request.recognitionLanguages = wanted.filter(supported.contains)
    }
    try? VNImageRequestHandler(cgImage: image, options: [:]).perform([request])
    var lines: [[VNRecognizedTextObservation]] = []
    for piece in (request.results ?? []).sorted(by: { $0.boundingBox.midY > $1.boundingBox.midY }) {
        if let last = lines.last?.first, abs(last.boundingBox.midY - piece.boundingBox.midY) < last.boundingBox.height / 2 {
            lines[lines.count - 1].append(piece)
        } else {
            lines.append([piece])
        }
    }
    return lines.map { line in
        line.sorted { $0.boundingBox.minX < $1.boundingBox.minX }
            .compactMap { $0.topCandidates(1).first?.string }
            .joined(separator: "  ")
    }.joined(separator: "\n")
}

/// picture opens an image file (its first frame), turned the way the camera held it.
func picture(_ url: URL) -> CGImage? {
    guard let source = CGImageSourceCreateWithURL(url as CFURL, nil) else { return nil }
    let options: [CFString: Any] = [
        kCGImageSourceCreateThumbnailFromImageAlways: true, kCGImageSourceThumbnailMaxPixelSize: 3000,
        kCGImageSourceCreateThumbnailWithTransform: true,
    ]
    return CGImageSourceCreateThumbnailAtIndex(source, 0, options as CFDictionary)
}

/// rendered draws a PDF page about 2000 pixels high, for Vision.
func rendered(_ page: PDFPage) -> CGImage? {
    let box = page.bounds(for: .mediaBox)
    let scale = 2000 / max(box.height, 1)
    let image = page.thumbnail(of: NSSize(width: box.width * scale, height: box.height * scale), for: .mediaBox)
    return image.cgImage(forProposedRect: nil, context: nil, hints: nil)
}

func text(of url: URL) -> String {
    if pictures.contains(url.pathExtension.lowercased()) {
        return picture(url).map(recognize) ?? ""
    }
    guard let doc = PDFDocument(url: url) else { return "" }
    var text = ""
    for i in 0..<min(doc.pageCount, maxPages) {
        text += (doc.page(at: i)?.string ?? "") + "\n"
    }
    if text.trimmingCharacters(in: .whitespacesAndNewlines).count >= 20 {
        return text
    }
    // No text layer: a scan. Vision reads its pages as pictures.
    var scanned = ""
    for i in 0..<min(doc.pageCount, scanPages) {
        if let page = doc.page(at: i), let image = rendered(page) {
            scanned += recognize(image) + "\n"
        }
    }
    return scanned
}

for path in CommandLine.arguments.dropFirst() {
    let block = "\u{1E}" + path + "\n" + text(of: URL(fileURLWithPath: path))
    FileHandle.standardOutput.write(block.data(using: .utf8) ?? Data())
}
