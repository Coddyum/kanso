// pdftext prints the text of the first pages of each PDF given on the command line.
// Each block starts with the record separator (0x1E), then the file path on its own line.
// Built on PDFKit, which ships with macOS: no dependency.
import Foundation
import PDFKit

let maxPages = 3

for path in CommandLine.arguments.dropFirst() {
    var text = ""
    if let doc = PDFDocument(url: URL(fileURLWithPath: path)) {
        for i in 0..<min(doc.pageCount, maxPages) {
            text += (doc.page(at: i)?.string ?? "") + "\n"
        }
    }
    let block = "\u{1E}" + path + "\n" + text
    FileHandle.standardOutput.write(block.data(using: .utf8) ?? Data())
}
