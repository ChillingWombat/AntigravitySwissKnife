"""
Antigravity Swiss Knife Process Package.
========================================
Process lifecycle management, singleton lock detection, and clean relauncher.
"""

from antigravity_swiss.process.lifecycle import ProcessLifecycleManager, ProcessManager, ProcessManagerProtocol
from antigravity_swiss.process.lock_manager import LockState, SingletonLockManager

__all__ = [
    "SingletonLockManager",
    "LockState",
    "ProcessLifecycleManager",
    "ProcessManager",
    "ProcessManagerProtocol",
]
