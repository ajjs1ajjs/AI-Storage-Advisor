import os
import time
from datetime import datetime, timedelta
from app.core.config import logger

class RulesEngine:
    @staticmethod
    def get_default_rules() -> list[dict]:
        """Returns the default cleanup rules list."""
        return [
            {
                "id": "temp_old",
                "name": "Temp files older than 30 days",
                "category": "temp",
                "condition": "older_than_days",
                "value": 30,
                "enabled": True
            },
            {
                "id": "log_large",
                "name": "Log files larger than 100 MB",
                "category": "log",
                "condition": "larger_than_mb",
                "value": 100,
                "enabled": True
            },
            {
                "id": "log_old",
                "name": "Log files older than 14 days",
                "category": "log",
                "condition": "older_than_days",
                "value": 14,
                "enabled": True
            },
            {
                "id": "backup_old",
                "name": "Backups older than 90 days",
                "category": "backup",
                "condition": "older_than_days",
                "value": 90,
                "enabled": True
            },
            {
                "id": "large_huge",
                "name": "Uncategorized files larger than 1 GB",
                "category": "large",
                "condition": "larger_than_mb",
                "value": 1024,
                "enabled": False
            }
        ]

    def __init__(self, rules: list[dict] = None):
        self.rules = rules if rules is not None else self.get_default_rules()

    def evaluate_file(self, file_info: dict) -> tuple[bool, str]:
        """Checks if a file matches any of the enabled rules.
        Returns (is_match, rule_name)."""
        category = file_info.get("category", "other")
        size = file_info.get("size", 0)
        
        # Calculate file age in days
        try:
            mtime_ts = file_info.get("last_modified_ts")
            if mtime_ts is not None:
                age_days = (time.time() - mtime_ts) / 86400.0
            else:
                mod_time_str = file_info.get("last_modified")
                mod_time = datetime.strptime(mod_time_str, "%Y-%m-%d %H:%M:%S")
                age_days = (datetime.now() - mod_time).days
        except Exception:
            age_days = 0

        for r in self.rules:
            if not r.get("enabled", True):
                continue
                
            rule_cat = r.get("category")
            # If the rule applies to this category, or is 'all'
            if rule_cat == "all" or rule_cat == category or (rule_cat == "large" and category in ("large", "other")):
                cond = r.get("condition")
                val = r.get("value", 0)
                
                if cond == "older_than_days":
                    if age_days >= val:
                        return True, r.get("name", "Matches age rule")
                elif cond == "larger_than_mb":
                    if size >= val * 1024 * 1024:
                        return True, r.get("name", "Matches size rule")
                        
        return False, ""

    def process_files(self, scan_results: dict) -> dict:
        """Processes scan results, flagging items that match enabled rules.
        Adds 'rule_matches' key with details to matching items.
        Returns the updated scan_results dict."""
        categories = ["large_files", "temp_files", "log_files", "backup_files", "cache_files"]
        
        matched_count = 0
        matched_size = 0

        for cat in categories:
            if cat in scan_results:
                for file_info in scan_results[cat]:
                    is_match, rule_name = self.evaluate_file(file_info)
                    if is_match:
                        file_info["rule_match"] = rule_name
                        matched_count += 1
                        matched_size += file_info.get("size", 0)
                    else:
                        file_info["rule_match"] = None

        scan_results["rules_flagged_count"] = matched_count
        scan_results["rules_flagged_size"] = matched_size
        logger.info(f"Rules engine completed. Flagged {matched_count} files for cleanup.")
        return scan_results
