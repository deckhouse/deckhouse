export default function FormatError(error) {
  // error messages created by this client:
  if (typeof error === 'string') {
    return error;
  }

  // axios error:
  if (error && error.response) {
    if (error.response.data) {
      let e = error.response.data.error;
      switch (typeof(e)) {
        case 'string':
          return e;
        case 'object':
          return JSON.stringify(e);
        default:
          return JSON.stringify(error.response.data);
      }
    } else {
      return String(error.response.status);
    }
  }

  console.error(error);
  return 'console.error';
}
