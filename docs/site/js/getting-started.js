$( document ).ready(function() {
    if ($.cookie("demotoken") ) {
        let username = 'demotoken';
        let password = $.cookie("demotoken");
        let registry = 'registry.deckhouse.io';
        let auth = btoa(username + ':' + password);
        let config = '{"auths": { "'+ registry +'": { "username": "'+ username +'", "password": "' + password + '", "auth": "' + auth +'"}}}';
        let matchString = '<YOUR_ACCESS_STRING_IS_HERE>';
        let matchedElements = $('.details code span.s').filter(function () {
            return this.innerText == matchString;
        }).text(btoa(config));
    } else {
        console.log("No demotoken, so InitConfiguration was not updated");
    }
});