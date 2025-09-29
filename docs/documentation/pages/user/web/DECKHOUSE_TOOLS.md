---
title: "Web interface for downloading Deckhouse utilities"
permalink: en/user/web/deckhouse-tools.html
---

Web interface for downloading Deckhouse utilities provides centralized access to CLI tools for popular operating systems (Linux, macOS, Windows). Using it, you can quickly locate and download executable binaries for the required architecture and version, as well as receive notifications about their updates. This functionality is implemented by the `deckhouse-tools` module.

## Accessing the web interface

1. To open the web interface, enter `tools.<CLUSTER_NAME_TEMPLATE>` in your browser’s address bar, where `<CLUSTER_NAME_TEMPLATE>` is the string matching the cluster’s DNS name template, as specified by the global parameter `modules.publicDomainTemplate`. The exact URL format may vary depending on system configuration. Check with your administrator for the correct URL.
1. On first login, enter your user credentials. After successful authentication, the Deckhouse Tools download page will open, presenting links to download the Deckhouse CLI tool for various operating systems: Linux, macOS, and Windows.

   ![Opening the Deckhouse Tools web interface](../../images/deckhouse-tools/deckhouse-tools.png)

1. Select the required version, download, and install the corresponding executable file.
