$( document ).ready(function() {
    if ($.cookie("demotoken") ) {
        let username = 'demotoken';
        let password = $.cookie("demotoken");
        let registry = 'registry.deckhouse.io';
        let auth = btoa(username + ':' + password);
        let config = '{"auths": { "'+ registry +'": { "username": "'+ username +'", "password": "' + password + '", "auth": "' + auth +'"}}}';
        let matchStringClusterConfig = '<YOUR_ACCESS_STRING_IS_HERE>';
        // let matchStringDockerLogin = 'docker login -u demotoken -p <ACCESS_TOKEN> registry.deckhouse.io';
        let matchStringDockerLogin = '"<ACCESS_TOKEN>"';
        $('.details code span.s').filter(function () {
            return this.innerText == matchStringClusterConfig;
        }).text(btoa(config));
        $('.highlight code span.s2').filter(function () {
            return this.innerText == matchStringDockerLogin;
        }).text(password);
    } else {
        console.log("No demotoken, so InitConfiguration was not updated");
    }
});