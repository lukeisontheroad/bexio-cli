export default {
  extends: ["@commitlint/config-conventional"],
  // Dependabot writes "ci: Bump x from 1 to 2", which trips subject-case.
  // Its messages are generated, not authored, so exempt them instead of
  // relaxing the rule for everyone.
  ignores: [(message) => /^Signed-off-by: dependabot\[bot\]/m.test(message)],
};
