import axios, { AxiosResponse } from 'axios';
import debug, { Debugger } from 'debug';
import * as R from 'ramda';
import { from, Observable } from 'rxjs';

import { HttpErrorImpl } from './HttpError';

interface HttpClientOptions {
  allowConcurrentRequests?: boolean;
  auth?: HttpBasicAuth;
  root: string;
  slowDownRequests?: boolean;
}

interface HttpBasicAuth {
  password: string;
  username: string;
}

interface GetOptions {
  auth?: HttpBasicAuth;
  qs?: Record<string, unknown>;
}

interface PostOptions extends GetOptions {
  body?: Record<string, unknown>;
}

export interface HttpResponse<T> {
  body: T;
  response: Record<string, unknown>;
}

enum HttpMethod {
  DELETE = 'delete',
  GET = 'get',
  POST = 'post',
  PUT = 'put',
}

// Accept-Encoding needed because of https://github.com/axios/axios/issues/5346
const headers = { accept: 'application/json', 'Accept-Encoding': 'gzip,deflate,compress' };

type HttpMethodCall<T> = (path: string, options?: PostOptions) => Observable<HttpResponse<T>>;

export class HttpClient {
  public get: <T>(url: string, options?: GetOptions) => Observable<HttpResponse<T>> =
    this.serverCall(HttpMethod.GET);
  public post: <T>(url: string, options?: PostOptions) => Observable<HttpResponse<T>> =
    this.serverCall(HttpMethod.POST);
  public put: <T>(url: string, options?: PostOptions) => Observable<HttpResponse<T>> =
    this.serverCall(HttpMethod.PUT);

  private options: HttpClientOptions;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  private networkQueue: Promise<any>;
  private debug: Debugger;

  constructor(options: HttpClientOptions) {
    this.debug = debug(`httpClient:${options.root}`);
    this.options = options;
    this.networkQueue = Promise.resolve();
  }

  private serverCall<T>(method: HttpMethod): HttpMethodCall<T> {
    return <T>(path: string, options: PostOptions = {}): Observable<HttpResponse<T>> => {
      const createRequest = (): Promise<HttpResponse<T>> => {
        this.debug(`Making ${method} call to ${path}`);
        return new Promise((resolve, reject) =>
          R.composeP(
            R.compose(
              (data: HttpResponse<T>) => {
                if (this.options.slowDownRequests) {
                  setTimeout(() => {
                    resolve(data);
                  }, 1000);
                } else {
                  resolve(data);
                }
                return Promise.resolve();
              },
              R.tap(() => {
                this.debug(`Finished ${method} call to ${path}`);
              }),
              (response: AxiosResponse) => ({
                body: response.data,
                response: response,
              }),
            ),
            axios.request,
          )({
            auth: options.auth || this.options.auth,
            data: options.body,
            headers,
            responseType: 'json',
            method,
            params: options.qs,
            url: `${this.options.root}${path}`,
          }).catch((error: NodeJS.ErrnoException) => {
            if (axios.isAxiosError(error) && error.response) {
              reject(
                new HttpErrorImpl(
                  error.response.statusText,
                  error.response.status,
                  error.response.data,
                ),
              );
            } else {
              this.debug(`Failed ${method} call to ${path}`);
              // TODO Transform error
              reject(error);
            }
          }),
        );
      };
      if (this.options.allowConcurrentRequests) {
        return from(createRequest());
      }

      let outerResolve: (value: HttpResponse<T>) => void;
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      let outerReject: (reason?: any) => void;

      const promiseToReturn = new Promise<HttpResponse<T>>((resolve, reject) => {
        outerReject = reject;
        outerResolve = resolve;
      });

      this.networkQueue = this.networkQueue.then(() => {
        return createRequest().then(outerResolve).catch(outerReject);
      });
      return from(promiseToReturn);
    };
  }
}
