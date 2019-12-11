module.exports = {
  roots: ['<rootDir>/src', '<rootDir>/test'],
  coverageDirectory: './coverage/',
  coverageReporters: ['html'],
  setupFilesAfterEnv: ['<rootDir>test/setupTests.ts'],
  preset: 'ts-jest',
  testRegex: '(/test/.*(test|spec))\\.(tsx?)$',
  moduleFileExtensions: ['ts', 'js', 'json', 'node'],
  moduleNameMapper: {
    'src/(.*)': '<rootDir>/src/$1',
    'test/(.*)': '<rootDir>/test/$1',
  },
  globals: {
    'ts-jest': {
      tsConfig: './test/tsconfig.json',
    },
  },
};
