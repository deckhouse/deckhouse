$( document ).ready(function() {
    if ($.cookie("demotoken") || $.cookie("license-token") ) {
        let username = 'license-token';
        let password = $.cookie("license-token") ? $.cookie("license-token") : $.cookie("demotoken");
        let registry = 'registry.deckhouse.io';
        let auth = btoa(username + ':' + password);
        let config = '{"auths": { "'+ registry +'": { "username": "'+ username +'", "password": "' + password + '", "auth": "' + auth +'"}}}';
        let matchStringClusterConfig = '<YOUR_ACCESS_STRING_IS_HERE>';
        let matchStringDockerLogin = "<LICENSE_TOKEN>";

        $('.details code span.s').filter(function () {
            return this.innerText == matchStringClusterConfig;
        }).text(btoa(config));

        $('.highlight code').filter(function () {
            return this.innerText.match(matchStringDockerLogin) == matchStringDockerLogin;
        }).each(function(index) {
            $(this).text($(this).text().replace(matchStringDockerLogin,password));
        });
    } else {
        console.log("No license token, so InitConfiguration was not updated");
    }
});
