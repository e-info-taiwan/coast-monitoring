// Test-only PostgreSQL WASM bridge over stdio: no listener, credentials or live DB.
import { PGlite } from "@electric-sql/pglite"
import { pgcrypto } from "@electric-sql/pglite/contrib/pgcrypto"
import { citext } from "@electric-sql/pglite/contrib/citext"
import fs from "node:fs/promises"
const db = new PGlite({ extensions: { pgcrypto, citext } })
await db.waitReady
const dir = new URL("../../migrations/", import.meta.url)
for (const file of (await fs.readdir(dir)).filter((f) => f.endsWith(".sql")).sort()) {
  await db.exec("BEGIN;" + (await fs.readFile(new URL(file, dir), "utf8")) + ";COMMIT;")
}
function frame(type, data) {
  const out = Buffer.alloc(data.length + 5)
  out[0] = type.charCodeAt(0)
  out.writeInt32BE(data.length + 4, 1)
  data.copy(out, 5)
  return out
}
let startup = true,
  pending = Buffer.alloc(0)
for await (const chunk of process.stdin) {
  pending = Buffer.concat([pending, chunk])
  while (pending.length >= (startup ? 4 : 5)) {
    const length = pending.readInt32BE(startup ? 0 : 1) + (startup ? 0 : 1)
    if (pending.length < length) break
    const message = pending.subarray(0, length)
    pending = pending.subarray(length)
    if (startup) {
      startup = false
      process.stdout.write(frame("R", Buffer.alloc(4)))
      for (const [key, value] of Object.entries({
        server_version: "17.0",
        client_encoding: "UTF8",
        server_encoding: "UTF8",
        DateStyle: "ISO, MDY",
        TimeZone: "UTC",
        integer_datetimes: "on",
        standard_conforming_strings: "on",
      }))
        process.stdout.write(frame("S", Buffer.from(key + "\0" + value + "\0")))
      process.stdout.write(frame("K", Buffer.alloc(8)))
      process.stdout.write(frame("Z", Buffer.from("I")))
    } else if (message[0] === 88) {
      await db.close()
      process.exit(0)
    } else {
      process.stdout.write(await db.execProtocolRaw(message))
    }
  }
}
await db.close()
