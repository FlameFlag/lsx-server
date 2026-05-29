import AppKit
import SwiftUI

@main
struct Lemonade2KeygenApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) private var appDelegate
    @StateObject private var model = KeygenModel(helperPath: helperPathFromArguments())

    var body: some Scene {
        WindowGroup {
            ContentView(model: model)
        }
        .windowResizability(.contentSize)

        WindowGroup("How It Works", id: "about") {
            AboutView()
                .preferredColorScheme(model.appearance.colorScheme)
        }
        .defaultSize(width: 1000, height: 820)
    }
}

final class AppDelegate: NSObject, NSApplicationDelegate {
    func applicationDidFinishLaunching(_ notification: Notification) {
        NSApp.setActivationPolicy(.regular)
        NSApp.activate(ignoringOtherApps: true)
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
        true
    }
}
