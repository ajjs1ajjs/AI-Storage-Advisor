from abc import ABC, abstractmethod

class AIProvider(ABC):
    def __init__(self, config: dict):
        self.config = config

    def get_recommendation_system_prompt(self) -> str:
        return (
            "You are an SRE Storage Analytics assistant. Help the user optimize their disk space. "
            "You MUST analyze the target Operating System and Connection Type provided in the request to adjust your recommendations accordingly (be extremely cautious with critical system folders like C:\\Windows on Windows or /boot, /etc, /var on Linux). "
            "You MUST write your response entirely in Ukrainian language. "
            "For each specific file path you recommend to delete, you MUST append a markdown link next to it in the exact format: "
            "[Видалити](delete://<absolute_path>). For example: "
            "'- C:\\Users\\Admin\\AppData\\Local\\Temp\\test.log (12.4 MB) - [Видалити](delete://C:/Users/Admin/AppData/Local/Temp/test.log)'. "
            "CRITICAL: You MUST NOT generate delete:// links for system files, libraries (.dll, .so), application binaries (.exe), database files, configuration files, or active application data/models (like Ollama libraries, Chrome models, pagefile.sys, game files) even if they take up a lot of space, as deleting them can break the target operating system or applications. Only generate delete:// links for temporary files (Temp) and log files (Log) that are 100% safe to delete without breaking any software."
        )

    @abstractmethod
    def test_connection(self) -> tuple[bool, str]:
        """Test connection to the AI provider.
        Returns (success, message)."""
        pass

    @abstractmethod
    def generate_recommendations(self, disk_summary: str, files_list: list) -> str:
        """Generate storage cleanup recommendations.
        Returns markdown text."""
        pass

    @abstractmethod
    def explain_folder(self, folder_path: str, details: dict) -> str:
        """Explain why a specific folder is suggested for cleanup and estimate risks.
        Returns markdown text."""
        pass

    def get_available_models(self) -> list[str]:
        """Fetch list of available models from the provider.
        Returns a list of model names."""
        return []
