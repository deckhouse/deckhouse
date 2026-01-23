function prepare_parameters_update() {
    // Helper function to update placeholders in code blocks
    function updatePlaceholder(placeholder, storageKey, defaultValue) {
        const value = sessionStorage.getItem(storageKey) || defaultValue;
        if (!value) return;
        
        const placeholderRegex = new RegExp(placeholder.replace(/[<>]/g, '\\$&'), 'g');
        
        // Update code span elements (for syntax-highlighted code blocks)
        $('code span').filter(function () {
            return ((this.innerText.match(placeholderRegex) || []).length > 0);
        }).each(function (index) {
            let content = ($(this)[0]) ? $(this)[0].innerText : null;
            if (content && content.length > 0) {
                $(this)[0].innerText = content.replace(placeholderRegex, value);
            }
        });
        
        // Update full code blocks (textContent for full code blocks)
        $('code').each(function (index) {
            let content = ($(this)[0]) ? $(this)[0].textContent : null;
            if (content && content.length > 0 && content.indexOf(placeholder) !== -1) {
                $(this)[0].textContent = content.replace(placeholderRegex, value);
            }
        });
    }

    // Update domain parameters (complex regex replacements)
    let domainPattern = sessionStorage.getItem('dhctl-domain');
    let domainSuffix = domainPattern ? domainPattern.replace('%s\.', '') : null;

    if (domainSuffix && domainPattern && domainSuffix.length > 0) {
        // Update rendered code block
        $('code span').filter(function () {
            return ((this.innerText.match(/[\S]+\.example\.com/i) || []).length > 0);
        }).each(function (index) {
            let content = ($(this)[0]) ? $(this)[0].innerText : null;
            if (content && content.length > 0) {
                $(this)[0].innerText = content.replace(/([\S]+)\.example\.com/i, domainPattern.replace('%s', content.match(/([\S]+)\.example\.com/i)[1]));
            }
        });

        // Updating snippet
        $('[example-hosts]').each(function (index) {
            let content = ($(this)[0]) ? $(this)[0].textContent : null;
            if (content && content.length > 0) {
                content.match(/([\S]+)\.example\.com/ig).forEach(function (item, index, arr) {
                    let serviceDomain = item.match(/([\S]+)\.example\.com/i)[1];
                    content = content.replace(/[\S]+.example\.com/i, domainPattern.replace('%s', serviceDomain));
                });
                $(this)[0].textContent = content;
            }
        });
    }

    // Update NFS parameters
    updatePlaceholder('<NFS_SHARE>', 'dhctl-nfs-share', '/srv/nfs/dvp');
    updatePlaceholder('<NFS_HOST>', 'dhctl-nfs-host', '192.168.1.100');

    // Update subnet parameters
    updatePlaceholder('<POD_SUBNET_CIDR>', 'dhctl-pod-subnet-cidr', '10.115.0.0/16');
    updatePlaceholder('<SERVICE_SUBNET_CIDR>', 'dhctl-service-subnet-cidr', '10.225.0.0/16');

    // Update optional subnet parameters (only if value exists)
    const internalNetworkCIDRs = sessionStorage.getItem('dhctl-internal-network-cidrs');
    if (internalNetworkCIDRs && internalNetworkCIDRs.length > 0) {
        updatePlaceholder('<INTERNAL_NETWORK_CIDRS>', 'dhctl-internal-network-cidrs', null);
    }

    const virtualMachineCIDRs = sessionStorage.getItem('dhctl-virtual-machine-cidrs');
    if (virtualMachineCIDRs && virtualMachineCIDRs.length > 0) {
        updatePlaceholder('<VIRTUAL_MACHINE_CIDRS>', 'dhctl-virtual-machine-cidrs', null);
    }
}

document.addEventListener("DOMContentLoaded", function() {
    prepare_parameters_update();
});
