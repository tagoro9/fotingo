import { compose, converge, prop, reduce, replace, toPairs } from 'ramda';

/**
 * Given a template and some data, replace every reference in the template to any of the keys in the data
 * with the value
 */
export const parseTemplate: <T extends string>(options: {
  data: Record<T, string>;
  template: string;
}) => string = converge(
  reduce((message: string, [k, v]: string[]) => replace(`{${k}}`, v, message)),
  [prop('template'), compose(toPairs, prop('data'))],
);
