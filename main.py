import sys
from PySide6.QtWidgets import QApplication
from app.ui.main_window import MainWindow
from app.core.config import logger

def main():
    logger.info("Starting AI Storage Advisor GUI Application...")
    app = QApplication(sys.argv)
    app.setStyle("Fusion") # Use clean Fusion style as baseline
    
    window = MainWindow()
    window.show()
    
    sys.exit(app.exec())

if __name__ == "__main__":
    main()
