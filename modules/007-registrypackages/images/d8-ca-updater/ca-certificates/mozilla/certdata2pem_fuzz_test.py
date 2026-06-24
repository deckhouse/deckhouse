#!/usr/bin/env python3

import atheris
import os
import sys
import tempfile

with atheris.instrument_imports():
    from certdata2pem_fuzzable import convert_certdata_text


def TestOneInput(data: bytes) -> None:
    if len(data) < 8 or len(data) > 65536:
        return

    text = data.decode("utf-8", errors="ignore")

    with tempfile.TemporaryDirectory() as tmpdir:
        old_cwd = os.getcwd()

        try:
            os.chdir(tmpdir)
            convert_certdata_text(text)
        except (
            KeyError,
            NotImplementedError,
            UnicodeEncodeError,
            UnicodeDecodeError,
            ValueError,
            TypeError,
            OSError,
        ):
            pass
        finally:
            os.chdir(old_cwd)


def main() -> None:
    atheris.Setup(sys.argv, TestOneInput)
    atheris.Fuzz()


if __name__ == "__main__":
    main()
