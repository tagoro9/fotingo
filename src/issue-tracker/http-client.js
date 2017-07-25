import request from 'request';
import R from 'ramda';
import { debug } from '../util';

const handleServerResponse = (resolve, reject) =>
  R.ifElse(
    R.compose(R.not, R.isNil, R.nthArg(0)),
    reject,
    R.ifElse(
      R.compose(R.propSatisfies(R.gt(400), 'statusCode'), R.nthArg(1)),
      R.unapply(R.compose(resolve, args => ({ response: args[1], body: args[2] }))),
      R.compose(
        reject,
        R.unapply(args => {
          R.compose(
            debug('http'),
            R.concat('Request failed with status code '),
            R.prop('statusCode'),
            R.nth(1),
          )(args);
          return args[2];
        }),
      ),
    ),
  );

const headers = { accept: 'application/json' };

export default function(rootUrl) {
  const makeUrl = R.concat(rootUrl);
  let auth = {};
  const jar = request.jar();
  const serverCall = R.curry(
    (method, url, { form, body }) =>
      new Promise((resolve, reject) => {
        debug(
          'http',
          `Performing ${method} ${makeUrl(url)} ${body
            ? `with body ${JSON.stringify(body, null, 2)}`
            : ''}`,
        );
        return request(
          { url: makeUrl(url), body, form, headers, jar, json: true, method, auth },
          handleServerResponse(resolve, reject),
        );
      }),
  );
  const setCookieToJar = R.compose(
    R.bind(R.partialRight(jar.setCookie, [rootUrl]), jar),
    request.cookie,
  );

  return {
    post: serverCall('POST'),
    get: serverCall('GET', R.__, {}),
    setAuth: ({ login, password }) => {
      auth = { ...auth, user: login, pass: password };
    },
    setCookie: R.ifElse(R.is(Array), R.map(setCookieToJar), setCookieToJar),
  };
}
