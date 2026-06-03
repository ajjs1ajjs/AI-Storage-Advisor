from abc import ABC, abstractmethod

class AIProvider(ABC):
    def __init__(self, config: dict):
        self.config = config

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
