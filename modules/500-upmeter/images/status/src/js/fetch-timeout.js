export const fetchWithTimeout = function(url, timeout ) {
  return new Promise( (resolve, reject) => {
    // Set timeout timer
    let timer = setTimeout(
      () => reject( new Error('Request timed out') ),
      timeout
    );

    fetch( url ).then(
      response => resolve( response ),
      err => reject( err )
    ).finally( () => clearTimeout(timer) );
  })
}
