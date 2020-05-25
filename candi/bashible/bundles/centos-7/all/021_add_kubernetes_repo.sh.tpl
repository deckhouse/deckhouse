if bb-yum-repo? d8-kubernetes/x86_64; then
  exit 0
fi

cat > /etc/yum.repos.d/d8-kubernetes.repo << "EOF"
[d8-kubernetes]
name=Kubernetes
baseurl=https://packages.cloud.google.com/yum/repos/kubernetes-el7-x86_64
enabled=1
gpgcheck=1
repo_gpgcheck=1
gpgkey=file:///etc/pki/rpm-gpg/d8-kubernetes-yum file:///etc/pki/rpm-gpg/d8-kubernetes-rpm-package
EOF

cat > /etc/pki/rpm-gpg/d8-kubernetes-yum << EOF
-----BEGIN PGP PUBLIC KEY BLOCK-----
Version: GnuPG v1

mQENBFUd6rIBCAD6mhKRHDn3UrCeLDp7U5IE7AhhrOCPpqGF7mfTemZYHf/5Jdjx
cOxoSFlK7zwmFr3lVqJ+tJ9L1wd1K6P7RrtaNwCiZyeNPf/Y86AJ5NJwBe0VD0xH
TXzPNTqRSByVYtdN94NoltXUYFAAPZYQls0x0nUD1hLMlOlC2HdTPrD1PMCnYq/N
uL/Vk8sWrcUt4DIS+0RDQ8tKKe5PSV0+PnmaJvdF5CKawhh0qGTklS2MXTyKFoqj
XgYDfY2EodI9ogT/LGr9Lm/+u4OFPvmN9VN6UG+s0DgJjWvpbmuHL/ZIRwMEn/tp
uneaLTO7h1dCrXC849PiJ8wSkGzBnuJQUbXnABEBAAG0QEdvb2dsZSBDbG91ZCBQ
YWNrYWdlcyBBdXRvbWF0aWMgU2lnbmluZyBLZXkgPGdjLXRlYW1AZ29vZ2xlLmNv
bT6JAT4EEwECACgFAlUd6rICGy8FCQWjmoAGCwkIBwMCBhUIAgkKCwQWAgMBAh4B
AheAAAoJEDdGwginMXsPcLcIAKi2yNhJMbu4zWQ2tM/rJFovazcY28MF2rDWGOnc
9giHXOH0/BoMBcd8rw0lgjmOosBdM2JT0HWZIxC/Gdt7NSRA0WOlJe04u82/o3OH
WDgTdm9MS42noSP0mvNzNALBbQnlZHU0kvt3sV1YsnrxljoIuvxKWLLwren/GVsh
FLPwONjw3f9Fan6GWxJyn/dkX3OSUGaduzcygw51vksBQiUZLCD2Tlxyr9NvkZYT
qiaWW78L6regvATsLc9L/dQUiSMQZIK6NglmHE+cuSaoK0H4ruNKeTiQUw/EGFaL
ecay6Qy/s3Hk7K0QLd+gl0hZ1w1VzIeXLo2BRlqnjOYFX4CZAQ0EWsFo2wEIAOsX
XwoJuxmWjg2MC9V5xMEKenpZwFAnmhKHv4T3yNf1jOdQKs2uCZ4JwIxS9MNEPF9N
oMnJtoe6B9trjeeqGRs2knjthewhr5gvp4QT16ZKZC2OtJYiJj7ZgljCwOCyByQX
d26qRvTY50FCWHohsc+hcHof/9vU+BliyiYH7zjVdbUtIk9iVhsitZ/AN9C+2QVA
j3Svo2SdVNCWmpCHkYs1Y1ipE2sZA+awH42tRiuSXWdS3UtEa76sJ7htJpKY1vAo
xAqRE4TiROIHvYM+TvMfgubS6jRgUVYbiqwwi6oSKEn/0o1fwZgGv61aDIuiguWx
0reX7h1Wp3xyOQkzUTEAEQEAAbRAR29vZ2xlIENsb3VkIFBhY2thZ2VzIEF1dG9t
YXRpYyBTaWduaW5nIEtleSA8Z2MtdGVhbUBnb29nbGUuY29tPokBPgQTAQIAKAUC
WsFo2wIbLwUJBaOagAYLCQgHAwIGFQgCCQoLBBYCAwECHgECF4AACgkQagMLIboH
9Pvx7wf/VYfYs3+dU2GblNLVVgkbwH4hbzNLgGrKjPEL2IkAmpkhUdeXyDxr8e6z
xF9dHtydgdyDyyNJol9CGo71Fsqd9+K5CAaurBDG4LaMFroz9ArN6NN4/QyCLrun
Kssk1asUjvVGGuK1BmbNNnY+hbF+/pv5O/m/Ss9ob663Unjumf6RiC1Rop2wnPW6
aLofMroBpwN/QLQKSwl0obsw5axlwHjF47Eli7Lo247opx0TPz9fIRSMi4g6WFhN
3SEfwT9IQFtdd+3v9UFALnA2rjSLM+L7pYUr97U7jYMinNDvj2iBhDV6h17E82Ev
N6QpHdeEas1cn3mvko7XRWuwsU13wg==
=4CNh
-----END PGP PUBLIC KEY BLOCK-----
EOF

cat > /etc/pki/rpm-gpg/d8-kubernetes-rpm-package << EOF
-----BEGIN PGP PUBLIC KEY BLOCK-----
Version: GnuPG v1

mQENBFWKtqgBCADmKQWYQF9YoPxLEQZ5XA6DFVg9ZHG4HIuehsSJETMPQ+W9K5c5
Us5assCZBjG/k5i62SmWb09eHtWsbbEgexURBWJ7IxA8kM3kpTo7bx+LqySDsSC3
/8JRkiyibVV0dDNv/EzRQsGDxmk5Xl8SbQJ/C2ECSUT2ok225f079m2VJsUGHG+5
RpyHHgoMaRNedYP8ksYBPSD6sA3Xqpsh/0cF4sm8QtmsxkBmCCIjBa0B0LybDtdX
XIq5kPJsIrC2zvERIPm1ez/9FyGmZKEFnBGeFC45z5U//pHdB1z03dYKGrKdDpID
17kNbC5wl24k/IeYyTY9IutMXvuNbVSXaVtRABEBAAG0Okdvb2dsZSBDbG91ZCBQ
YWNrYWdlcyBSUE0gU2lnbmluZyBLZXkgPGdjLXRlYW1AZ29vZ2xlLmNvbT6JATgE
EwECACIFAlWKtqgCGy8GCwkIBwMCBhUIAgkKCwQWAgMBAh4BAheAAAoJEPCcOUw+
G6jV+QwH/0wRH+XovIwLGfkg6kYLEvNPvOIYNQWnrT6zZ+XcV47WkJ+i5SR+QpUI
udMSWVf4nkv+XVHruxydafRIeocaXY0E8EuIHGBSB2KR3HxG6JbgUiWlCVRNt4Qd
6udC6Ep7maKEIpO40M8UHRuKrp4iLGIhPm3ELGO6uc8rks8qOBMH4ozU+3PB9a0b
GnPBEsZdOBI1phyftLyyuEvG8PeUYD+uzSx8jp9xbMg66gQRMP9XGzcCkD+b8w1o
7v3J3juKKpgvx5Lqwvwv2ywqn/Wr5d5OBCHEw8KtU/tfxycz/oo6XUIshgEbS/+P
6yKDuYhRp6qxrYXjmAszIT25cftb4d4=
=/PbX
-----END PGP PUBLIC KEY BLOCK-----
EOF

rpmkeys --import /etc/pki/rpm-gpg/d8-kubernetes-yum
rpmkeys --import /etc/pki/rpm-gpg/d8-kubernetes-rpm-package

yum -q makecache -y --disablerepo='*' --enablerepo=d8-kubernetes
