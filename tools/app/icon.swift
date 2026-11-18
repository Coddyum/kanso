// Draws the app icon (1024 px PNG): a dark squircle, a « C » drawn as an open
// indigo ring, and the green dot the calendar uses for a paid due date.
import AppKit

let px = 1024
let rep = NSBitmapImageRep(
    bitmapDataPlanes: nil, pixelsWide: px, pixelsHigh: px, bitsPerSample: 8, samplesPerPixel: 4,
    hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0)!
let ctx = NSGraphicsContext(bitmapImageRep: rep)!.cgContext
let space = CGColorSpace(name: CGColorSpace.sRGB)!

func color(_ hex: UInt32, _ alpha: CGFloat = 1) -> CGColor {
    CGColor(
        srgbRed: CGFloat(hex >> 16 & 255) / 255, green: CGFloat(hex >> 8 & 255) / 255,
        blue: CGFloat(hex & 255) / 255, alpha: alpha)
}
func gradient(_ colors: [CGColor]) -> CGGradient {
    CGGradient(colorsSpace: space, colors: colors as CFArray, locations: nil)!
}

/// The macOS icon shape: a superellipse, whose corners curve continuously.
func squircle(_ r: CGRect, n: CGFloat = 5) -> CGPath {
    let path = CGMutablePath()
    for i in 0...1440 {
        let t = CGFloat(i) / 1440 * 2 * .pi
        let (c, s) = (cos(t), sin(t))
        let point = CGPoint(
            x: r.midX + r.width / 2 * copysign(pow(abs(c), 2 / n), c),
            y: r.midY + r.height / 2 * copysign(pow(abs(s), 2 / n), s))
        i == 0 ? path.move(to: point) : path.addLine(to: point)
    }
    path.closeSubpath()
    return path
}

let box = CGRect(x: 100, y: 100, width: 824, height: 824)
let shape = squircle(box)
let center = CGPoint(x: 512, y: 512)

// Body: a soft drop shadow, a graphite gradient, a faint indigo light behind the ring.
ctx.saveGState()
ctx.setShadow(offset: CGSize(width: 0, height: -14), blur: 36, color: color(0x000000, 0.45))
ctx.addPath(shape)
ctx.setFillColor(color(0x141518))
ctx.fillPath()
ctx.restoreGState()
ctx.saveGState()
ctx.addPath(shape)
ctx.clip()
ctx.drawLinearGradient(
    gradient([color(0x2d2f37), color(0x101114)]), start: CGPoint(x: 512, y: 924), end: CGPoint(x: 512, y: 100),
    options: [])
ctx.drawRadialGradient(
    gradient([color(0x7178f0, 0.20), color(0x7178f0, 0)]), startCenter: center, startRadius: 0, endCenter: center,
    endRadius: 380, options: [])
ctx.restoreGState()

// Rim: a thin light edge, bright at the top, gone by the middle.
ctx.saveGState()
ctx.addPath(shape)
ctx.clip()
ctx.addPath(shape.copy(strokingWithWidth: 6, lineCap: .round, lineJoin: .round, miterLimit: 10))
ctx.clip()
ctx.drawLinearGradient(
    gradient([color(0xffffff, 0.24), color(0xffffff, 0)]), start: CGPoint(x: 512, y: 924), end: CGPoint(x: 512, y: 560),
    options: [])
ctx.restoreGState()

// The « C »: an open ring, its gap on the right.
let radius: CGFloat = 172
let arc = CGMutablePath()
arc.addArc(center: center, radius: radius, startAngle: 50 * .pi / 180, endAngle: 310 * .pi / 180, clockwise: false)
let ring = arc.copy(strokingWithWidth: 92, lineCap: .round, lineJoin: .round, miterLimit: 10)
ctx.saveGState()
ctx.setShadow(offset: .zero, blur: 46, color: color(0x7c83f2, 0.55))
ctx.addPath(ring)
ctx.setFillColor(color(0x7c83f2))
ctx.fillPath()
ctx.restoreGState()
ctx.saveGState()
ctx.addPath(ring)
ctx.clip()
ctx.drawLinearGradient(
    gradient([color(0xb7baff), color(0x7c83f2), color(0x9a6bff)]), start: CGPoint(x: 330, y: 720),
    end: CGPoint(x: 690, y: 320), options: [])
ctx.restoreGState()

// The dot, in the gap: green, like a paid due date in the calendar.
let dot = CGRect(x: center.x + radius - 50, y: center.y - 50, width: 100, height: 100)
ctx.saveGState()
ctx.setShadow(offset: .zero, blur: 30, color: color(0x4cc38a, 0.6))
ctx.setFillColor(color(0x4cc38a))
ctx.fillEllipse(in: dot)
ctx.restoreGState()
ctx.saveGState()
ctx.addEllipse(in: dot)
ctx.clip()
let shine = CGPoint(x: dot.midX - 18, y: dot.midY + 22)
ctx.drawRadialGradient(
    gradient([color(0xffffff, 0.45), color(0xffffff, 0)]), startCenter: shine, startRadius: 0, endCenter: shine,
    endRadius: 60, options: [])
ctx.restoreGState()

try! rep.representation(using: .png, properties: [:])!.write(to: URL(fileURLWithPath: CommandLine.arguments[1]))
