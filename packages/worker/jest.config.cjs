const { createDefaultPreset } = require("ts-jest");

const tsJestTransformCfg = createDefaultPreset().transform;

module.exports = {
  testEnvironment: "node",
  transform: {
    ...tsJestTransformCfg,
  },
  testMatch: ["**/tests/**/*.test.ts"],
  collectCoverageFrom: [
    "src/**/*.ts",
    "!**/*.test.ts",
    "!**/*.spec.ts"
  ],
  coverageDirectory: "coverage",
  coverageReporters: ["lcov", "text", "text-summary"]
};
