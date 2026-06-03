import time
import sqlite3
import psutil
from datetime import datetime
from app.database.db_manager import db
from app.core.config import logger

class ForecastEngine:
    def __init__(self, profile_id: int):
        self.profile_id = profile_id

    def calculate_forecast(self, scan_path: str) -> dict:
        """Calculates storage trend and exhaustion timeline based on SQLite scan history.
        Returns forecast report dict."""
        with db.connection() as conn:
            cursor = conn.cursor()
            
            # 1. Fetch scan history sorted by time
            cursor.execute(
                "SELECT scan_time, total_size FROM scan_history "
                "WHERE profile_id = ? ORDER BY scan_time ASC",
                (self.profile_id,)
            )
            rows = cursor.fetchall()
        
        if len(rows) < 2:
            return {
                "status": "insufficient_data",
                "message": "Needs at least 2 scan history data points to project storage trends.",
                "days_remaining": -1,
                "daily_growth_bytes": 0,
                "trend_points": []
            }

        # 2. Extract x (timestamp seconds) and y (size in bytes)
        points = []
        first_time = None
        
        for r in rows:
            try:
                # SQLite timestamp parsing (handles default CURRENT_TIMESTAMP format '%Y-%m-%d %H:%M:%S')
                dt = datetime.strptime(r["scan_time"], "%Y-%m-%d %H:%M:%S")
                ts = dt.timestamp()
            except Exception:
                continue
                
            if first_time is None:
                first_time = ts
            
            # Days since first scan
            days = (ts - first_time) / 86400.0
            points.append((days, r["total_size"], r["scan_time"]))

        if len(points) < 2:
            return {
                "status": "insufficient_data",
                "message": "Insufficient valid chronological data points.",
                "days_remaining": -1,
                "daily_growth_bytes": 0,
                "trend_points": []
            }

        # 3. Simple Linear Regression (Ordinary Least Squares)
        n = len(points)
        sum_x = sum(p[0] for p in points)
        sum_y = sum(p[1] for p in points)
        sum_xy = sum(p[0] * p[1] for p in points)
        sum_x2 = sum(p[0] ** 2 for p in points)

        denominator = (n * sum_x2) - (sum_x ** 2)
        if abs(denominator) < 1e-6:
            # All scans happened at the exact same second
            # Fallback: simple delta between first and last
            first, last = points[0], points[-1]
            time_delta_days = last[0] - first[0]
            if time_delta_days > 1e-4:
                slope = (last[1] - first[1]) / time_delta_days
            else:
                slope = 0
        else:
            slope = ((n * sum_xy) - (sum_x * sum_y)) / denominator

        daily_growth = slope # Bytes per day
        
        # 4. Get Current Disk Space using psutil
        days_remaining = -1
        status = "stable"
        message = "Storage consumption is stable or shrinking."
        
        try:
            usage = psutil.disk_usage(scan_path)
            free_bytes = usage.free
            total_bytes = usage.total
            
            if daily_growth > 0:
                days_remaining = int(free_bytes / daily_growth)
                status = "exhaustion_risk" if days_remaining < 90 else "normal_growth"
                message = f"Disk will be exhausted in estimated {days_remaining} days."
        except Exception as e:
            logger.error(f"Failed to fetch disk usage: {e}")
            free_bytes = 0
            total_bytes = 0

        # Save to database history
        try:
            with db.connection() as conn:
                cursor = conn.cursor()
                cursor.execute(
                    "INSERT INTO forecast_history (profile_id, predicted_days_to_full, growth_rate_bytes_day) "
                    "VALUES (?, ?, ?)",
                    (self.profile_id, days_remaining, int(daily_growth))
                )
        except Exception as e:
            logger.debug(f"Failed to log forecast history: {e}")

        # Format points for graph mapping: list of (scan_time, size, projected_size)
        trend_points = []
        for p in points:
            # y_projected = y_intercept + slope * x
            # y_intercept = (sum_y - slope * sum_x) / n
            y_intercept = (sum_y - slope * sum_x) / n
            projected = y_intercept + slope * p[0]
            trend_points.append({
                "days": p[0],
                "scan_time": p[2],
                "actual_size": p[1],
                "projected_size": projected
            })

        return {
            "status": status,
            "message": message,
            "days_remaining": days_remaining,
            "daily_growth_bytes": int(daily_growth),
            "free_bytes": free_bytes,
            "total_bytes": total_bytes,
            "trend_points": trend_points
        }
