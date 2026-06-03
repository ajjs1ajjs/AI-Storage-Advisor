import os
import send2trash
from app.database.db_manager import db
from app.core.config import logger

class CleanupAdvisor:
    def __init__(self, profile_id: int):
        self.profile_id = profile_id

    def format_size(self, size_bytes: int) -> str:
        for unit in ['B', 'KB', 'MB', 'GB', 'TB']:
            if size_bytes < 1024.0:
                return f"{size_bytes:.2f} {unit}"
            size_bytes /= 1024.0
        return f"{size_bytes:.2f} PB"

    def dry_run(self, file_paths: list[str]) -> dict:
        """Analyzes which files can be deleted and calculates total size to free.
        Returns dry run report dict."""
        total_count = 0
        total_size = 0
        writable_paths = []
        restricted_paths = []

        for path in file_paths:
            path = os.path.abspath(path)
            if not os.path.exists(path):
                continue
                
            total_count += 1
            try:
                size = os.path.getsize(path)
                # Check write (delete) permissions
                if os.access(path, os.W_OK):
                    writable_paths.append((path, size))
                    total_size += size
                else:
                    restricted_paths.append((path, size))
            except Exception:
                restricted_paths.append((path, 0))

        return {
            "total_count": total_count,
            "total_size": total_size,
            "total_size_formatted": self.format_size(total_size),
            "writable_files": writable_paths,
            "restricted_files": restricted_paths,
            "can_proceed": len(writable_paths) > 0
        }

    def safe_delete(self, file_paths: list[str], use_recycle_bin: bool = True) -> tuple[int, int, list]:
        """Deletes files, either sending them to the Recycle Bin or deleting permanently.
        Writes success/failure history to database.
        Returns (deleted_count, size_freed_bytes, list_of_failed_paths)."""
        deleted_count = 0
        size_freed = 0
        failed_paths = []

        with db.connection() as conn:
            cursor = conn.cursor()

            for path in file_paths:
                path = os.path.abspath(path)
                if not os.path.exists(path):
                    continue
                
                try:
                    size = os.path.getsize(path)
                    if use_recycle_bin:
                        # Move to Trash
                        send2trash.send2trash(path)
                    else:
                        # Permanent delete
                        os.remove(path)
                    
                    deleted_count += 1
                    size_freed += size
                    
                    # Log success in DB
                    cursor.execute(
                        "INSERT INTO cleanup_history (profile_id, cleaned_path, size_freed, status) VALUES (?, ?, ?, 'success')",
                        (self.profile_id, path, size)
                    )
                    logger.info(f"Safely deleted file: {path} (Freed: {self.format_size(size)})")
                except Exception as e:
                    failed_paths.append((path, str(e)))
                    logger.error(f"Failed to delete file {path}: {e}")
                    
                    # Log failure in DB
                    cursor.execute(
                        "INSERT INTO cleanup_history (profile_id, cleaned_path, size_freed, status, error_message) VALUES (?, ?, 0, 'failed', ?)",
                        (self.profile_id, path, str(e))
                    )

        return deleted_count, size_freed, failed_paths
