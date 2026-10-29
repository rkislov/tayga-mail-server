// Independent Apple CryptoKit wire-format check, matching the primitive and
// combined IV/ciphertext/tag layout used by Monal's MLCrypto.swift.
// Test data only. This is not a launch or end-to-end test of the Monal app.
import Foundation
import CryptoKit

let input = try JSONDecoder().decode([String: String].self, from: FileHandle.standardInput.readDataToEndOfFile())
func bytes(_ name: String) throws -> Data {
    guard let value = input[name], let data = Data(base64Encoded: value) else {
        throw NSError(domain: "Invalid fixture", code: 1)
    }
    return data
}
let key = SymmetricKey(data: try bytes("key"))
let combined = try bytes("iv") + bytes("wire")
let plain = try AES.GCM.open(AES.GCM.SealedBox(combined: combined), using: key)
let nativeKey = SymmetricKey(size: .bits256)
let sealed = try AES.GCM.seal(plain, using: nativeKey)
let result = [
    "plain": plain.base64EncodedString(),
    "key": nativeKey.withUnsafeBytes { Data($0).base64EncodedString() },
    "iv": Data(sealed.nonce).base64EncodedString(),
    "wire": (sealed.ciphertext + sealed.tag).base64EncodedString()
]
FileHandle.standardOutput.write(try JSONEncoder().encode(result))
