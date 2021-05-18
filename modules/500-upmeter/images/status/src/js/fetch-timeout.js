export const fetchWithTimeout = function (url, timeout) {
  return new Promise((resolve, reject) => {
    // Set timeout timer
    const timer = setTimeout(() => reject(new Error("Request timed out")), timeout)

    fetch(url)
      .then(resolve, reject)
      .finally(() => clearTimeout(timer))
  })
}
