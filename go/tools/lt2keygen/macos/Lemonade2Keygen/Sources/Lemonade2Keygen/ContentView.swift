import SwiftUI

struct ContentView: View {
    @Environment(\.openWindow) private var openWindow
    @StateObject var model: KeygenModel

    var body: some View {
        VStack(alignment: .leading, spacing: 24) {
            header
            registrationSection
            outputSection
            statusLine
        }
        .padding(34)
        .frame(width: 640, height: 430)
        .background(Color(nsColor: .windowBackgroundColor))
        .preferredColorScheme(model.appearance.colorScheme)
        .toolbar { toolbar }
    }

    private var header: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text("Lemonade Tycoon 2: New York Edition Keygen")
                .font(.title.bold())
            Text("Generate a registration name and activation key.")
                .font(.callout)
                .foregroundStyle(.secondary)
        }
    }

    private var registrationSection: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("Registration")
                .font(.headline)

            HStack(spacing: 10) {
                TextField("Optional registration name", text: $model.registrationInput)
                    .textFieldStyle(.roundedBorder)
                    .font(.system(.body, design: .monospaced))
                    .onSubmit { model.generate() }

                Button {
                    model.generate()
                } label: {
                    Text("Generate")
                        .frame(width: 72)
                }
                .buttonStyle(.borderedProminent)
                .controlSize(.large)
                .keyboardShortcut(.defaultAction)
            }
        }
    }

    private var outputSection: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Activation Pair")
                .font(.headline)

            if model.hasGenerated {
                VStack(alignment: .leading, spacing: 12) {
                    OutputRow(label: "Registration name", value: model.registrationName) {
                        model.copy(model.registrationName, label: "Registration name")
                    }
                    OutputRow(label: "Activation key", value: model.activationKey) {
                        model.copy(model.activationKey, label: "Activation key")
                    }
                }
            } else {
                EmptyResultView()
            }
        }
    }

    @ToolbarContentBuilder
    private var toolbar: some ToolbarContent {
        ToolbarItem(placement: .automatic) {
            Button("About") { openWindow(id: "about") }
        }
        ToolbarItem(placement: .automatic) {
            Picker("Appearance", selection: $model.appearance) {
                ForEach(Appearance.allCases) { appearance in
                    Text(appearance.title).tag(appearance)
                }
            }
            .pickerStyle(.segmented)
            .frame(width: 210)
        }
    }

    @ViewBuilder
    private var statusLine: some View {
        if !model.status.isEmpty {
            Text(model.status)
                .font(.caption)
                .foregroundStyle(.secondary)
                .lineLimit(2)
                .frame(maxWidth: .infinity, alignment: .leading)
        }
    }
}
