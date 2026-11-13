import assert from 'node:assert/strict'
import test from 'node:test'
import { encodeBase64, formatBytes, isUploadCancel, uploadPercent } from './upload.js'

test('formatBytes uses the shortest unit that still reads clearly', () => {
  assert.equal(formatBytes(0), '0 B')
  assert.equal(formatBytes(512), '512 B')
  assert.equal(formatBytes(1536), '1.5 KB')
  assert.equal(formatBytes(12.4 * 1024), '12.4 KB')
  assert.equal(formatBytes(5 * 1024 * 1024), '5 MB')
  assert.equal(formatBytes(128 * 1024 * 1024), '128 MB')
})

test('upload percent reserves the last point until the server finishes', () => {
  assert.equal(uploadPercent('read', 0), 0)
  assert.equal(uploadPercent('encode', 0), 0)
  assert.equal(uploadPercent('encode', 0.5), 6)
  assert.equal(uploadPercent('encode', 1), 12)
  assert.equal(uploadPercent('upload', 0), 12)
  assert.equal(uploadPercent('upload', 0.5), 56)
  assert.equal(uploadPercent('upload', 1), 99)
  assert.equal(uploadPercent('import', 1), 99)
  assert.equal(uploadPercent('done', 1), 100)
})

test('encodeBase64 reports progress and can be cancelled', async () => {
  const bytes = new Uint8Array([0xe4, 0xb8, 0xad, 65])
  const seen = []
  const encoded = await encodeBase64(bytes, (done) => seen.push(done))
  assert.deepEqual(seen, [bytes.length])
  assert.equal(atob(encoded), String.fromCharCode(...bytes))

  const signal = { aborted: true }
  await assert.rejects(encodeBase64(bytes, null, signal), (error) => isUploadCancel(error))
})
