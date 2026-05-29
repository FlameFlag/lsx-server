import AppKit
import SwiftUI

@MainActor
final class KeygenModel: ObservableObject {
    @Published var registrationInput = ""
    @Published var registrationName = ""
    @Published var activationKey = ""
    @Published var keyFormat = ""
    @Published var status = ""
    @Published var isGenerating = false
    @Published var appearance = Appearance.system
    @Published var hasGenerated = false

    private let helperPath: String

    init(helperPath: String) {
        self.helperPath = helperPath
    }

    func generate() {
        isGenerating = true
        status = ""

        let requestedName = registrationInput.trimmingCharacters(in: .whitespacesAndNewlines)
        Task.detached { [helperPath] in
            do {
                let response = try runHelper(helperPath: helperPath, name: requestedName)
                await MainActor.run {
                    self.registrationName = response.registrationName
                    self.activationKey = response.activationKey
                    self.keyFormat = response.keyFormat
                    self.status = ""
                    self.hasGenerated = true
                    self.isGenerating = false
                }
            } catch {
                await MainActor.run {
                    self.status = "Key generation failed: \(error.localizedDescription)"
                    self.isGenerating = false
                }
            }
        }
    }

    func copy(_ value: String, label: String) {
        guard !value.isEmpty else {
            status = "Nothing to copy for \(label.lowercased())."
            return
        }
        NSPasteboard.general.clearContents()
        NSPasteboard.general.setString(value, forType: .string)
        status = "\(label) copied."
    }
}

enum Appearance: Int, CaseIterable, Identifiable {
    case system
    case light
    case dark

    var id: Int { rawValue }

    var title: String {
        switch self {
        case .system: "System"
        case .light: "Light"
        case .dark: "Dark"
        }
    }

    var colorScheme: ColorScheme? {
        switch self {
        case .system: nil
        case .light: .light
        case .dark: .dark
        }
    }
}
