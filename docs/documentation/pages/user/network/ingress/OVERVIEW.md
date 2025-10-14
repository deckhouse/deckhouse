---
title: "Overview"
permalink: en/user/network/ingress/
---

Network Load Balancer (NLB) and Application Load Balancer (ALB) are used to provide external access to applications
deployed in a cluster managed by Deckhouse Kubernetes Platform.

## NLB features and purpose

NLB operates at the transport layer. It balances TCP and UDP traffic at the IP and port level.

Key advantages:

- High performance
- Minimal latency during traffic transmission
- Simple configuration

NLB is suitable for applications that use TCP/UDP protocols, such as databases.

## ALB features and purpose

ALB operates at the application layer. It analyzes the contents of incoming requests
(for example, HTTP headers, URL paths, cookies) and can route traffic based on them.

Advantages of ALB:

- Support for HTTP(S) protocols and gRPC
- Flexible routing (path-based, host-based)
- Ability to terminate SSL/TLS
- Integration with authentication and authorization mechanisms

ALB is suitable for web applications, APIs, and other services
where intelligent routing and working with HTTP requests are important.
