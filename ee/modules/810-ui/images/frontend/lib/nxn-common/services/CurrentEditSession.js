function LoadCurrentEditSession() {
  // TODO: raise smth if it's not in sessionStorage
  var json_data = sessionStorage.getItem('current_edit_session');
  if (json_data) {
    return JSON.parse(json_data);
  } else {
    return { id: uuid.v4() };
  }
}

// TODO: reload value from sessionStorage
var CurrentEditSession;
export default CurrentEditSession = LoadCurrentEditSession();
