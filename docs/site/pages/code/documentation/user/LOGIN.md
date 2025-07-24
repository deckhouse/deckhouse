---
title: "Login"
permalink: en/code/documentation/user/login.html
---

1. Request your access credentials from the Deckhouse Code administrator.  

1. Open the Deckhouse Code web interface at an address such as `https://code.example.com`.

   The web interface domain is generated based on the template specified in the [`publicDomainTemplate`](/documentation.html#publicdomaintemplate) parameter.

   If this parameter is not specified, Ingress resources will not be created. For testing purposes, you can use [sslip.io](https://sslip.io) or a similar service if you do not have access to wildcard DNS records.

   Please note:
   - The domain from the template must not match or be a subdomain of `clusterDomain`.
   - DNS must be properly configured both in the node networks and in the client networks.
   - If the template domain matches the node network zone, use only A records to assign addresses to the platform’s frontend web interfaces (e.g., zone `company.my`, template `%s.company.my`).

1. Log in using your existing credentials.
