/** @type {import('ts-jest').JestConfigWithTsJest} */
module.exports = {
  testEnvironment: 'node',
  testMatch: ['**/tests/**/*.test.ts'],
  transform: {
    '^.+\\.ts$': ['ts-jest', {
      useESM: true,
      diagnostics: false,
      tsconfig: {
        module: 'esnext',
        moduleResolution: 'node',
      },
    }],
  },
  moduleNameMapper: {
    '^(\\.?.*)\\.js$': '$1',
  },
  extensionsToTreatAsEsm: ['.ts'],
  coverageThreshold: {
    global: {
      statements: 95,
      branches: 95,
      functions: 95,
      lines: 95,
    },
  },
  collectCoverageFrom: [
    'src/**/*.ts',
    '!src/**/*.d.ts',
    '!src/mocks/**',
  ],
};
