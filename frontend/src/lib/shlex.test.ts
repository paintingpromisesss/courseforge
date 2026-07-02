import { describe, it, expect } from 'vitest';
import { splitArgs, joinArgs } from './shlex';

describe('shlex round-trip', () => {
  const cases: string[][] = [
    ['go', 'run', '.'],
    ['python3', '-m', 'pytest', '-v', 'main_test.py'],
    ['sh', '-c', 'g++ -std=c++17 -O2 -o cf-run main.cpp && ./cf-run'],
    ['sh', '-c', 'javac -cp .:/usr/share/java/junit4.jar main.java && java SolutionTest'],
    ['npx', '--yes', 'mocha', '--reporter', 'tap', 'main_test.js'],
  ];

  it('preserves argv through join → split', () => {
    for (const argv of cases) {
      expect(splitArgs(joinArgs(argv))).toEqual(argv);
    }
  });

  it('splits plain commands on whitespace', () => {
    expect(splitArgs('go test -v .')).toEqual(['go', 'test', '-v', '.']);
  });

  it('keeps a quoted span as one arg', () => {
    expect(splitArgs('sh -c "a && b"')).toEqual(['sh', '-c', 'a && b']);
    expect(splitArgs("sh -c 'a && b'")).toEqual(['sh', '-c', 'a && b']);
  });

  it('quotes only args that need it', () => {
    expect(joinArgs(['go', 'run', '.'])).toBe('go run .');
    expect(joinArgs(['sh', '-c', 'a b'])).toBe('sh -c "a b"');
  });
});
