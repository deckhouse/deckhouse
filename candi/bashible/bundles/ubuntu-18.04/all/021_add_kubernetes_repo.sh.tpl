if bb-apt-repo? http://apt.kubernetes.io/; then
  exit 0
fi

bb-apt-key-add <<END
-----BEGIN PGP PUBLIC KEY BLOCK-----

mQENBFrBaNsBCADrF18KCbsZlo4NjAvVecTBCnp6WcBQJ5oSh7+E98jX9YznUCrN
rgmeCcCMUvTDRDxfTaDJybaHugfba43nqhkbNpJ47YXsIa+YL6eEE9emSmQtjrSW
IiY+2YJYwsDgsgckF3duqkb02OdBQlh6IbHPoXB6H//b1PgZYsomB+841XW1LSJP
YlYbIrWfwDfQvtkFQI90r6NknVTQlpqQh5GLNWNYqRNrGQPmsB+NrUYrkl1nUt1L
RGu+rCe4bSaSmNbwKMQKkROE4kTiB72DPk7zH4Lm0uo0YFFWG4qsMIuqEihJ/9KN
X8GYBr+tWgyLooLlsdK3l+4dVqd8cjkJM1ExABEBAAG0QEdvb2dsZSBDbG91ZCBQ
YWNrYWdlcyBBdXRvbWF0aWMgU2lnbmluZyBLZXkgPGdjLXRlYW1AZ29vZ2xlLmNv
bT6JATgEEwECACwFAlrBaNsJEGoDCyG6B/T7AhsPBQkFo5qABgsJCAcDAgYVCAIJ
CgsEFgIDAQAAJr4IAM5lgJ2CTkTRu2iw+tFwb90viLR6W0N1CiSPUwi1gjEKMr5r
0aimBi6FXiHTuX7RIldSNynkypkZrNAmTMM8SU+sri7R68CFTpSgAvW8qlnlv2iw
rEApd/UxxzjYaq8ANcpWAOpDsHeDGYLCEmXOhu8LmmpY4QqBuOCM40kuTDRd52PC
JE6b0V1t5zUqdKeKZCPQPhsS/9rdYP9yEEGdsx0V/Vt3C8hjv4Uwgl8Fa3s/4ag6
lgIf+4SlkBAdfl/MTuXu/aOhAWQih444igB+rvFaDYIhYosVhCxP4EUAfGZk+qfo
2mCY3w1pte31My+vVNceEZSUpMetSfwit3QA8EE=
=csu4
-----END PGP PUBLIC KEY BLOCK-----
END

bb-apt-repo-add deb http://apt.kubernetes.io/ kubernetes-xenial main
