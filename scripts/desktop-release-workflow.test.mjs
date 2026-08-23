import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { load } from "js-yaml";

const projectRoot = resolve(fileURLToPath(new URL("..", import.meta.url)));

test("tagged desktop releases use an ad-hoc preview when Apple credentials are absent", () => {
  const workflow = load(
    readFileSync(resolve(projectRoot, ".github/workflows/desktop-release.yml"), "utf8"),
  );
  const steps = workflow.jobs["release-macos"].steps;
  const step = (name) => steps.find((candidate) => candidate.name === name);

  const signing = step("Select release signing mode");
  assert.equal(signing.id, "signing");
  assert.match(signing.run, /mode=ad-hoc/);
  assert.match(signing.run, /mode=developer-id/);

  const preview = step("Build and publish ad-hoc preview");
  assert.equal(preview.if, "steps.signing.outputs.mode == 'ad-hoc'");
  assert.equal(preview.env.APPLE_SIGNING_IDENTITY, "-");
  assert.equal(preview.with.prerelease, true);

  const notarized = step("Build and publish notarized desktop release");
  assert.equal(notarized.if, "steps.signing.outputs.mode == 'developer-id'");

  const checksum = step("Publish verified checksum");
  assert.match(checksum.run, /WTS_RELEASE_SIGNING_MODE.*developer-id/);
  assert.match(checksum.run, /xcrun stapler validate/);

  for (const jobName of ["build-macos", "release-macos"]) {
    const rust = workflow.jobs[jobName].steps.find(
      (candidate) => candidate.name === "Use Rust 1.98",
    );
    assert.equal(rust.with.toolchain, "1.98.0");
  }
});
