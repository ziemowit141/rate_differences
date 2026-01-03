import Foundation
import PDFKit

let arguments = CommandLine.arguments
guard arguments.count > 1 else {
    fputs("Usage: extract.swift <pdf-path>\n", stderr)
    exit(1)
}

let path = arguments[1]
let url = URL(fileURLWithPath: path)

guard let document = PDFDocument(url: url) else {
    fputs("Failed to open PDF at \\(path)\\n", stderr)
    exit(2)
}

var output = ""
for pageIndex in 0..<document.pageCount {
    guard let page = document.page(at: pageIndex),
          let text = page.string else {
        continue
    }
    output += text + "\\n"
}

print(output)
