import SwiftUI

struct OutputRow: View {
    let label: String
    let value: String
    let copy: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 5) {
            Text(label)
                .font(.caption.weight(.semibold))
                .foregroundStyle(.secondary)
            HStack(spacing: 10) {
                TextField("", text: .constant(value))
                    .textFieldStyle(.roundedBorder)
                    .font(.system(.body, design: .monospaced))
                    .lineLimit(1)
                Button("Copy", action: copy)
                    .disabled(value.isEmpty)
            }
        }
    }
}

struct EmptyResultView: View {
    var body: some View {
        VStack(spacing: 8) {
            Image(systemName: "key.horizontal")
                .font(.title2)
                .foregroundStyle(.secondary)
            Text("No activation key yet")
                .font(.callout.weight(.semibold))
            Text("Enter a name or leave it blank, then click Generate.")
                .font(.caption)
                .foregroundStyle(.secondary)
        }
        .frame(maxWidth: .infinity, minHeight: 126)
        .background(.quaternary.opacity(0.35), in: RoundedRectangle(cornerRadius: 12, style: .continuous))
    }
}
