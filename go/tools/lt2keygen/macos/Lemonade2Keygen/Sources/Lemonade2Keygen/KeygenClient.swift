import Foundation

struct KeygenResponse: Decodable {
    let registrationName: String
    let activationKey: String
    let keyFormat: String
    let note: String

    enum CodingKeys: String, CodingKey {
        case registrationName = "registration_name"
        case activationKey = "activation_key"
        case keyFormat = "key_format"
        case note
    }
}

func helperPathFromArguments() -> String {
    let args = CommandLine.arguments
    if let index = args.firstIndex(of: "--helper"), args.indices.contains(index + 1) {
        return args[index + 1]
    }
    return "lt2keygen"
}

func runHelper(helperPath: String, name: String) throws -> KeygenResponse {
    let process = Process()
    process.executableURL = URL(fileURLWithPath: helperPath)
    process.arguments = name.isEmpty ? ["-json"] : ["-json", "-name", name]

    let output = Pipe()
    let error = Pipe()
    process.standardOutput = output
    process.standardError = error

    try process.run()
    process.waitUntilExit()

    let data = output.fileHandleForReading.readDataToEndOfFile()
    if process.terminationStatus != 0 {
        let errorData = error.fileHandleForReading.readDataToEndOfFile()
        let message = String(data: errorData, encoding: .utf8)?.trimmingCharacters(in: .whitespacesAndNewlines)
        throw NSError(domain: "Lemonade2Keygen", code: Int(process.terminationStatus), userInfo: [
            NSLocalizedDescriptionKey: message?.isEmpty == false ? message! : "Helper exited with status \(process.terminationStatus)"
        ])
    }
    return try JSONDecoder().decode(KeygenResponse.self, from: data)
}
