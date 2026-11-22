// Later run steps get these through GITHUB_ENV; the token is masked in the logs.
const fs = require("node:fs");

const names = ["ACTIONS_RUNTIME_TOKEN", "ACTIONS_RESULTS_URL", "ACTIONS_CACHE_URL", "ACTIONS_CACHE_SERVICE_V2"];
const lines = [];
for (const name of names) {
  const value = process.env[name];
  if (value) {
    lines.push(`${name}=${value}`);
  }
}

console.log(`::add-mask::${process.env.ACTIONS_RUNTIME_TOKEN}`);
fs.appendFileSync(process.env.GITHUB_ENV, lines.join("\n") + "\n");
