jest.mock('../../src/util');
jest.mock('../../src/reporter');
import {
  ControlledError,
  throwControlledError,
  handleError,
  catchPromiseAndThrow,
} from '../../src/error';
import { error, debug, debugCurried } from '../../src/util';
import reporter from '../../src/reporter';

describe('Error', () => {
  describe('ControlledError', () => {
    test('replaces placeholders with the passed parameters', () => {
      const param = 'value';
      const contollerError = new ControlledError('message with {param}', {
        param,
        anotherParam: 'value',
      });
      expect(contollerError.message).toBe('message with value');
    });
  });

  test('throws an instance of ControlledError', () => {
    const throwError = throwControlledError('message', {});
    expect(throwError).toThrowError(ControlledError);
  });

  describe('handleError', () => {
    test('exits with error code = 1 if the error is unknown', () => {
      const exit = jest.fn();
      process.exit = exit;
      const e = new Error('This is an unknown error');
      handleError(e);
      expect(exit).toHaveBeenCalledWith(1);
      expect(error).toHaveBeenCalledWith(e);
    });

    test('exits with error code = 0 if error is a Controlled Error', () => {
      const exit = jest.fn();
      process.exit = exit;
      const e = new ControlledError('error message');
      handleError(e);
      expect(exit).toHaveBeenLastCalledWith(0);
      expect(error).toHaveBeenLastCalledWith(e.message);
    });

    test("exits with error code = 0 if error message is 'cancelled'", () => {
      const exit = jest.fn();
      process.exit = exit;
      const e = new Error('canceled');
      handleError(e);
      expect(reporter.log).toHaveBeenCalledWith('Hasta la vista baby!', 'wave');
    });
  });

  describe('catchPrimiseAndThrow', () => {
    test('shows the error and throws a ControlledError with the result of the passed function', () => {
      const e = new Error('error message');
      const promise = Promise.reject(e);
      const transformedMessage = 'This is the {param}';
      const parameters = { param: 'value' };
      const module = 'the-module';
      const transformmer = () => transformedMessage;
      return catchPromiseAndThrow(module, transformmer, parameters)(
        promise,
      ).catch(controlledError => {
        expect(debug).toHaveBeenCalledWith(module, e);
        expect(controlledError).toHaveProperty('message', 'This is the value');
      });
    });

    test('shows the error and throws a ControlledError otherwise', () => {
      const e = new Error('error message');
      const promise = Promise.reject(e);
      const errorMessage = 'The error message';
      const module = 'the-module';
      debugCurried.mockImplementation(() => () => {});
      return catchPromiseAndThrow(module, errorMessage)(promise).catch(controllerError => {
        expect(debugCurried).toHaveBeenCalledWith(module, e);
        expect(controllerError).toHaveProperty('message', errorMessage);
      });
    });
  });
});
