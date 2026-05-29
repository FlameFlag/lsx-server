import SwiftUI

struct ExplainerCard<Content: View>: View {
    let number: String
    let title: String
    @ViewBuilder let content: Content

    var body: some View {
        HStack(alignment: .top, spacing: 14) {
            Text(number)
                .font(.headline.monospacedDigit())
                .foregroundStyle(.white)
                .frame(width: 30, height: 30)
                .background(.tint, in: Circle())

            VStack(alignment: .leading, spacing: 12) {
                Text(title)
                    .font(.title3.bold())
                VStack(alignment: .leading, spacing: 12) {
                    content
                        .font(.body)
                        .fixedSize(horizontal: false, vertical: true)
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
        }
        .padding(16)
        .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 14, style: .continuous))
        .overlay {
            RoundedRectangle(cornerRadius: 14, style: .continuous)
                .stroke(.separator.opacity(0.35), lineWidth: 1)
        }
    }
}

struct FormulaBlock: View {
    let lines: [String]

    var body: some View {
        VStack(alignment: .leading, spacing: 5) {
            ForEach(lines, id: \.self) { line in
                Text(line)
                    .fixedSize(horizontal: false, vertical: true)
            }
        }
        .font(.system(.body, design: .monospaced))
        .textSelection(.enabled)
        .padding(12)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(.quaternary.opacity(0.35), in: RoundedRectangle(cornerRadius: 8, style: .continuous))
    }
}

struct ExampleBlock: View {
    let title: String
    let text: String

    init(title: String, body: String) {
        self.title = title
        self.text = body
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 5) {
            Label(title, systemImage: "lightbulb")
                .font(.caption.weight(.semibold))
                .foregroundStyle(.secondary)
            Text(text)
                .font(.callout)
                .fixedSize(horizontal: false, vertical: true)
        }
        .textSelection(.enabled)
        .padding(12)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 8, style: .continuous))
        .overlay {
            RoundedRectangle(cornerRadius: 8, style: .continuous)
                .stroke(.separator.opacity(0.25), lineWidth: 1)
        }
    }
}

struct InteractiveCard<Content: View>: View {
    let title: String
    let caption: String
    @ViewBuilder let content: Content

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Label(title, systemImage: "slider.horizontal.3")
                .font(.headline)
            Text(caption)
                .font(.caption)
                .foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)
            content
        }
        .padding(14)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 12, style: .continuous))
        .overlay {
            RoundedRectangle(cornerRadius: 12, style: .continuous)
                .stroke(.separator.opacity(0.30), lineWidth: 1)
        }
    }
}
