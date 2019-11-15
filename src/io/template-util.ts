import { compose, converge, prop, reduce, replace, toPairs } from 'ramda';

/**
 * Given a template and some data, replace every reference in the template to any of the keys in the data
 * with the value
 */
export const parseTemplate: <T extends string>(options: {
  template: string;
  data: Record<T, string>;
}) => string = converge(
  reduce((msg: string, [k, v]: string[]) => replace(`{${k}}`, v, msg)),
  [prop('template'), compose(toPairs, prop('data'))],
);
