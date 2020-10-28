import 'bootstrap/dist/css/bootstrap.min.css'

import {fetchWithTimeout} from './js/fetch-timeout'
import {updatePage} from './js/page'

document.addEventListener('DOMContentLoaded', function () {
  fetchCurrentStatus();
})

const fetchCurrentStatus = function() {
  updatePage("fetch start")
  fetchWithTimeout("/public/api/status", 5000)
    .then(
      response => {
        if (response.status === 500) {
          throw new Error("Upmeter API internal error");
        }
        if (!response.ok) {
          throw new Error("Upmeter API is not reachable: "+response.statusText);
        }
        return response.json()
      }, error => {
        throw new Error("Upmeter API is not reachable: "+error.message)
      })
    .then(
      json => {
        updatePage("fetch success", json)
      },
      error => {
        updatePage("fetch error", error)
      })
    .then(() => {
      //setTimeout(fetchCurrentStatus, 10000);
    })
}
