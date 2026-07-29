import { execFile } from "node:child_process";
import { writeFile } from "node:fs/promises";
import { promisify } from "node:util";

const run = promisify(execFile);

export async function publish(_config, context) {
  const notesPath = `${process.env.RUNNER_TEMP}/chansat-release-notes.md`;
  await writeFile(notesPath, context.nextRelease.notes, "utf8");

  const { stdout, stderr } = await run(
    "goreleaser",
    ["release", "--release-notes", notesPath, "--clean"],
    { maxBuffer: 10 * 1024 * 1024 },
  );

  if (stdout) context.logger.log(stdout);
  if (stderr) context.logger.log(stderr);
}
