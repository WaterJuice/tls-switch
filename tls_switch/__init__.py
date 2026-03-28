# ----------------------------------------------------------------------------------------
#   tls-switch
#   ----------
#
#   SNI-based TLS reverse proxy. This Python package is a thin launcher for
#   the Go binary which handles all logic.
#
#   (c) 2026 WaterJuice — Released under the Unlicense; see LICENSE.
# ----------------------------------------------------------------------------------------

from .version import VERSION_STR

__version__ = VERSION_STR
__all__ = ["__version__"]
