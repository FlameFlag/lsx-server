// swift-tools-version: 5.9

import PackageDescription

let package = Package(
    name: "Lemonade2Keygen",
    platforms: [.macOS(.v13)],
    products: [
        .executable(name: "Lemonade2Keygen", targets: ["Lemonade2Keygen"]),
    ],
    targets: [
        .executableTarget(name: "Lemonade2Keygen"),
    ]
)
