import React = require('react');

import { RequestProps, SelectRequestProps } from './props';
import { SelectRequest } from './SelectRequest';
import { TextRequest } from './TextRequest';

function isSelectRequest(props: RequestProps): props is SelectRequestProps {
  return 'options' in props.request;
}

export function InputRequest(props: RequestProps): JSX.Element {
  if (isSelectRequest(props)) {
    return <SelectRequest {...props} />;
  }
  return <TextRequest {...props} />;
}
