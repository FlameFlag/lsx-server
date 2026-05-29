# lt2keygen

`lt2keygen` generates naturally-valid Armadillo ShortV3 signed Lemonade2 registration key pairs.

## CLI

Running without arguments opens the default GUI backend that was compiled in. Passing a registration name or CLI flags keeps the command-line behavior.

```sh
go run ./tools/lt2keygen
go run ./tools/lt2keygen -name "Example User"
go run ./tools/lt2keygen -json "Example User"
go run ./tools/lt2keygen --help
```

## GTK4 UI (`diamondburned/gotk4`)

The GTK UI uses `github.com/diamondburned/gotk4/pkg`, behind the `gtk` build tag so the normal CLI build does not require GTK development libraries.

```sh
go run -tags gtk ./tools/lt2keygen -ui gtk
```

Native prerequisites from the upstream `gotk4-examples` docs:

```sh
# macOS
brew install gtk4 gobject-introspection pkg-config

# Ubuntu 21.04+
sudo apt install libgtk-4-dev

# Fedora
sudo dnf install gtk4-devel gobject-introspection-devel

# Windows/MSYS2
pacman -S mingw-w64-x86_64-toolchain mingw-w64-x86_64-gtk4 mingw-w64-x86_64-gobject-introspection
```

## SwiftUI UI (`tmc/swiftui`)

The SwiftUI UI uses `github.com/tmc/swiftui`, behind the `swiftui` build tag and only on macOS. The upstream package currently requires macOS 26 or later and the Swift toolchain in `PATH`.

```sh
go run -tags swiftui ./tools/lt2keygen -ui swiftui
```

## Windows UI (`rodrigocfd/windigo`)

The Windows native UI uses `github.com/rodrigocfd/windigo`, behind the `windigo` build tag and only on Windows. Windigo is pure Go and calls Win32 directly.

```sh
go run -tags windigo ./tools/lt2keygen -ui windows
go build -tags windigo -trimpath -ldflags "-s -w -H=windowsgui" ./tools/lt2keygen
```

## SwiftUI Alternatives

`github.com/tmc/apple/x/swiftui` can host an app-owned Swift package inside AppKit. That is useful if the UI should remain authored in Swift while Go owns the app shell, but this tool uses the direct `github.com/tmc/swiftui` binding as requested.
