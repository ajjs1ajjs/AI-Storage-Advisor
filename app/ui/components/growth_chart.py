from PySide6.QtWidgets import QWidget
from PySide6.QtCore import Qt, QPointF
from PySide6.QtGui import QPainter, QPen, QColor, QBrush, QPainterPath, QFont

class GrowthChart(QWidget):
    def __init__(self, parent=None):
        super().__init__(parent)
        self.points = [] # list of dicts: {"days": float, "actual_size": float, "projected_size": float}
        self.total_capacity = 0
        self.setStyleSheet("background-color: #13151a;")

    def set_data(self, points: list, total_capacity: int):
        self.points = points
        self.total_capacity = total_capacity
        self.update() # Trigger repaint

    def format_size(self, size_bytes: int) -> str:
        for unit in ['B', 'KB', 'MB', 'GB', 'TB']:
            if size_bytes < 1024.0:
                return f"{size_bytes:.1f} {unit}"
            size_bytes /= 1024.0
        return f"{size_bytes:.1f} PB"

    def paintEvent(self, event):
        painter = QPainter(self)
        painter.setRenderHint(QPainter.Antialiasing)
        
        width = self.width()
        height = self.height()
        
        # Margins for axes
        margin_left = 70
        margin_bottom = 40
        margin_right = 30
        margin_top = 20

        chart_width = width - margin_left - margin_right
        chart_height = height - margin_top - margin_bottom

        # Background grid and borders
        painter.setPen(QPen(QColor("#1f232b"), 1))
        painter.drawRect(margin_left, margin_top, chart_width, chart_height)
        
        if not self.points or len(self.points) < 2:
            # Draw "No Data" message
            painter.setPen(QColor("#64748b"))
            painter.setFont(QFont("Segoe UI", 12))
            painter.drawText(
                self.rect(), 
                Qt.AlignCenter, 
                "Scan multiple times to plot storage growth trend."
            )
            return

        # 1. Determine Min/Max Bounds
        xs = [p["days"] for p in self.points]
        ys_actual = [p["actual_size"] for p in self.points]
        ys_proj = [p["projected_size"] for p in self.points]
        
        min_x = min(xs)
        max_x = max(xs) if max(xs) > min_x else min_x + 1
        
        # Max Y bound is either max projected size or total capacity (whichever is larger, cap at capacity)
        max_y = max(max(ys_actual), max(ys_proj))
        if self.total_capacity > 0:
            max_y = min(max_y * 1.15, self.total_capacity)
            
        min_y = min(ys_actual) * 0.95 # offset to zoom in

        # Avoid zero division
        x_span = max_x - min_x if max_x > min_x else 1
        y_span = max_y - min_y if max_y > min_y else 1

        # Coordinate mappings
        def map_coords(x, y):
            px = margin_left + ((x - min_x) / x_span) * chart_width
            py = margin_top + chart_height - ((y - min_y) / y_span) * chart_height
            return QPointF(px, py)

        # 2. Draw Horizontal Gridlines (Y-axis)
        painter.setFont(QFont("Segoe UI", 9))
        grid_lines = 4
        for i in range(grid_lines + 1):
            val_y = min_y + (y_span * i / grid_lines)
            pt = map_coords(min_x, val_y)
            
            # Gridline
            painter.setPen(QPen(QColor("#1f232b"), 1, Qt.DashLine))
            painter.drawLine(margin_left, pt.y(), width - margin_right, pt.y())
            
            # Y Label
            painter.setPen(QColor("#94a3b8"))
            painter.drawText(
                10, pt.y() + 4, 
                margin_left - 15, 20, 
                Qt.AlignRight, 
                self.format_size(val_y)
            )

        # 3. Draw Vertical Timeline Gridlines (X-axis)
        for i in range(len(self.points)):
            p = self.points[i]
            pt = map_coords(p["days"], p["actual_size"])
            
            # Gridline
            painter.setPen(QPen(QColor("#1f232b"), 1, Qt.DashLine))
            painter.drawLine(pt.x(), margin_top, pt.x(), height - margin_bottom)
            
            # X Label (Draw date label relative to scan_time)
            # Just extract time/short date
            try:
                date_str = p["scan_time"].split()[0][5:] # Extract MM-DD
            except Exception:
                date_str = f"S{i+1}"
                
            painter.setPen(QColor("#94a3b8"))
            painter.drawText(
                pt.x() - 25, height - margin_bottom + 8,
                50, 20,
                Qt.AlignCenter,
                date_str
            )

        # 4. Draw Curves and Areas
        # Path for Area Gradient fill under the actual curve
        area_path = QPainterPath()
        area_path.moveTo(map_coords(self.points[0]["days"], min_y))
        
        # Path for the Solid Line curve
        line_path = QPainterPath()
        start_pt = map_coords(self.points[0]["days"], self.points[0]["actual_size"])
        line_path.moveTo(start_pt)
        area_path.lineTo(start_pt)

        # Plot actual line path
        for p in self.points[1:]:
            pt = map_coords(p["days"], p["actual_size"])
            line_path.lineTo(pt)
            area_path.lineTo(pt)

        # Close area path
        area_path.lineTo(map_coords(self.points[-1]["days"], min_y))
        area_path.closeSubpath()

        # Fill Area with Gradient
        gradient = QColor("#7c4dff")
        gradient_alpha = QColor(124, 77, 255, 30) # Translucent purple
        painter.setPen(Qt.NoPen)
        
        # We can build a simple linear brush
        painter.setBrush(QBrush(gradient_alpha))
        painter.drawPath(area_path)

        # Draw Actual Line (Solid vibrant purple)
        painter.setPen(QPen(QColor("#7c4dff"), 3, Qt.SolidLine))
        painter.setBrush(Qt.NoBrush)
        painter.drawPath(line_path)

        # 5. Draw Projected Trend Line (Dotted red/pink line)
        proj_path = QPainterPath()
        start_proj = map_coords(self.points[0]["days"], self.points[0]["projected_size"])
        proj_path.moveTo(start_proj)
        for p in self.points[1:]:
            pt = map_coords(p["days"], p["projected_size"])
            proj_path.lineTo(pt)

        painter.setPen(QPen(QColor("#f43f5e"), 2, Qt.DashLine))
        painter.drawPath(proj_path)

        # 6. Draw glowing point dots for actual scans
        for p in self.points:
            pt = map_coords(p["days"], p["actual_size"])
            
            # Outer glow dot
            painter.setBrush(QBrush(QColor(124, 77, 255, 90)))
            painter.setPen(Qt.NoPen)
            painter.drawEllipse(pt, 7.0, 7.0)
            
            # Inner solid dot
            painter.setBrush(QBrush(QColor("#a78bfa")))
            painter.drawEllipse(pt, 3.5, 3.5)
