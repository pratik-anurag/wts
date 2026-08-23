import assert from "node:assert/strict";
import { existsSync, mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const projectRoot = resolve(fileURLToPath(new URL("..", import.meta.url)));

test("go install creates the wts-ui command", () => {
  const temporaryRoot = mkdtempSync(join(tmpdir(), "wts-ui-command-"));
  const binaryDirectory = join(temporaryRoot, "bin");
  const result = spawnSync("go", ["install", "./cmd/wts-ui"], {
    cwd: projectRoot,
    encoding: "utf8",
    env: {
      ...process.env,
      GOBIN: binaryDirectory,
      GOCACHE: join(temporaryRoot, "cache"),
      GOMODCACHE: join(temporaryRoot, "modules"),
    },
  });

  assert.equal(result.status, 0, result.stderr);
  assert.equal(existsSync(join(binaryDirectory, "wts-ui")), true);
  assert.equal(existsSync(join(binaryDirectory, "wts-install")), false);
});
