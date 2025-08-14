function openTabAndSaveStatus(evt, linksClass, contentClass, contentId, storeKey = null, storeVal = null) {
    openTab(evt, linksClass, contentClass, contentId);
    if (storeKey && storeVal) {
        sessionStorage.setItem(storeKey, storeVal );
    }
}

function openTab(evt, linksClass, contentClass, contentId) {
  var i, tabcontent, tablinks;

  tabcontent = document.getElementsByClassName(contentClass);
  for (i = 0; i < tabcontent.length; i++) {
    tabcontent[i].style.display = "none";
  }

  tablinks = document.getElementsByClassName(linksClass);
  for (i = 0; i < tablinks.length; i++) {
    tablinks[i].className = tablinks[i].className.replace(" active", "");
  }

  document.getElementById(contentId).style.display = "block";
  evt.currentTarget.className += " active";
}
