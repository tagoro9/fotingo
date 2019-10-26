import debug, { Debugger } from 'debug';
import * as R from 'ramda';
import * as request from 'request';
import { from, Observable } from 'rxjs';
import { promisify } from 'util';

import { HttpErrorImpl } from './HttpError';

const requestAsPromise = promisify(request);

interface HttpClientOptions {
  root: string;
  allowConcurrentRequests?: boolean;
  slowDownRequests?: boolean;
  auth?: HttpBasicAuth;
}

interface HttpBasicAuth {
  user: string;
  pass: string;
}

interface GetOptions {
  qs?: object;
  auth?: HttpBasicAuth;
}

interface PostOptions extends GetOptions {
  form?: object;
  body?: object;
}

export interface HttpResponse<T> {
  response: object;
  body: T;
}

enum HttpMethod {
  GET = 'get',
  PUT = 'put',
  POST = 'post',
  DELETE = 'delete',
}

const headers = { accept: 'application/json' };

type HttpMethodCall<T> = (path: string, options?: PostOptions) => Observable<HttpResponse<T>>;

export class HttpClient {
  public get: <T>(
    url: string,
    options?: GetOptions,
  ) => Observable<HttpResponse<T>> = this.serverCall(HttpMethod.GET);
  public post: <T>(
    url: string,
    options?: PostOptions,
  ) => Observable<HttpResponse<T>> = this.serverCall(HttpMethod.POST);
  public put: <T>(
    url: string,
    options?: PostOptions,
  ) => Observable<HttpResponse<T>> = this.serverCall(HttpMethod.PUT);

  private options: HttpClientOptions;
  private networkQueue: Promise<any>;
  private debug: Debugger;

  constructor(options: HttpClientOptions) {
    this.debug = debug(`httpClient:${options.root}`);
    this.options = options;
    this.networkQueue = Promise.resolve();
  }

  private serverCall<T>(method: HttpMethod): HttpMethodCall<T> {
    return (path: string, options: PostOptions = {}) => {
      const createRequest = (): Promise<HttpResponse<T>> => {
        this.debug(`Making ${method} call to ${path}`);
        return new Promise((resolve, reject) =>
          R.composeP(
            R.ifElse(
              R.propSatisfies(R.gt(400), 'statusCode'),
              R.compose(
                (data: HttpResponse<T>) => {
                  if (this.options.slowDownRequests) {
                    setTimeout(() => {
                      resolve(data);
                    }, 1000);
                  } else {
                    resolve(data);
                  }
                },
                R.tap(() => {
                  this.debug(`Finished ${method} call to ${path}`);
                }),
                R.unapply(args => ({ response: args[0], body: args[0].body })),
              ),
              R.compose((response: request.Response) => {
                throw new HttpErrorImpl(response.statusMessage, response.statusCode, response.body);
              }),
            ),
            requestAsPromise,
          )({
            auth: options.auth || this.options.auth,
            body: options.body,
            form: options.form,
            headers,
            json: true,
            method,
            qs: options.qs,
            url: `${this.options.root}${path}`,
          }).catch((e: NodeJS.ErrnoException) => {
            this.debug(`Failed ${method} call to ${path}`);
            // TODO Transform error
            reject(e);
          }),
        );
      };
      if (this.options.allowConcurrentRequests) {
        return from(createRequest());
      }

      let outerResolve: (value: HttpResponse<T>) => void;
      let outerReject: (reason: any) => void;

      const promiseToReturn = new Promise<HttpResponse<T>>((resolve, reject) => {
        outerReject = reject;
        outerResolve = resolve;
      });

      this.networkQueue = this.networkQueue.then(() => {
        return createRequest()
          .then(outerResolve)
          .catch(outerReject);
      });
      return from(promiseToReturn);
    };
  }
}
