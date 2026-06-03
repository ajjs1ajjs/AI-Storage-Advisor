import requests
from app.providers.base import AIProvider
from app.core.config import logger

class OllamaProvider(AIProvider):
    def __init__(self, config: dict):
        super().__init__(config)
        self.base_url = config.get("base_url", "http://localhost:11434").rstrip("/")
        self.model = config.get("model", "llama3")

    def test_connection(self) -> tuple[bool, str]:
        try:
            # Query Ollama version or list tags to verify connection
            url = f"{self.base_url}/api/tags"
            response = requests.get(url, timeout=5)
            if response.status_code == 200:
                models = [m["name"] for m in response.json().get("models", [])]
                if self.model and any(self.model in m for m in models):
                    return True, f"Connected to Ollama! Available models: {', '.join(models[:3])}"
                return True, f"Connected to Ollama. Selected model '{self.model}' not found in: {models}"
            return False, f"Ollama returned status code {response.status_code}"
        except requests.exceptions.ConnectionError:
            return False, "Cannot connect to Ollama. Is it running?"
        except Exception as e:
            return False, f"Ollama connection error: {str(e)}"

    def _query(self, system_prompt: str, user_prompt: str) -> str:
        try:
            url = f"{self.base_url}/api/chat"
            payload = {
                "model": self.model,
                "messages": [
                    {"role": "system", "content": system_prompt},
                    {"role": "user", "content": user_prompt}
                ],
                "stream": False,
                "options": {
                    "temperature": float(self.config.get("temperature", 0.7))
                }
            }
            response = requests.post(url, json=payload, timeout=None)
            if response.status_code == 200:
                return response.json().get("message", {}).get("content", "Empty response")
            return f"Ollama Error (HTTP {response.status_code}): {response.text}"
        except Exception as e:
            logger.error(f"Ollama request error: {e}")
            return f"Ollama request failed: {e}"

    def generate_recommendations(self, disk_summary: str, files_list: list) -> str:
        system = (
            "You are an SRE Storage Analytics assistant. Help the user optimize their disk space. "
            "You MUST write your response entirely in Ukrainian language. "
            "For each specific file path you recommend to delete, you MUST append a markdown link next to it in the exact format: "
            "[Видалити](delete://<absolute_path>). For example: "
            "'- C:\\Users\\Admin\\AppData\\Local\\Temp\\test.log (12.4 MB) - [Видалити](delete://C:/Users/Admin/AppData/Local/Temp/test.log)'."
        )
        user = f"Review the disk state and suggest items to clean up.\nDisk Status:\n{disk_summary}\n\nTop Large / Temp / Log files found:\n"
        for idx, f in enumerate(files_list[:15]):
            user += f"- {f['path']} ({f['size_formatted']}) - Category: {f['category']}\n"
        user += "\nProvide clear, structured markdown recommendations, risks of deleting, and an actionable cleanup plan. You MUST write your response entirely in Ukrainian language."
        return self._query(system, user)

    def explain_folder(self, folder_path: str, details: dict) -> str:
        system = "You are an AI Storage Assistant explaining disk analysis results. You MUST write your response entirely in Ukrainian language."
        user = (
            f"Explain why this folder is marked for cleanup:\n"
            f"Folder: {folder_path}\n"
            f"Total Size: {details.get('size_formatted', 'unknown')}\n"
            f"File Types: {details.get('types', 'unknown')}\n"
            f"Last Accessed: {details.get('last_access', 'unknown')}\n"
            f"Risk Level: {details.get('risk_score', 'medium')}/100\n\n"
            f"Explain what this folder contains, what risks are associated with deleting it, "
            f"and what could be the potential savings. You MUST write your response entirely in Ukrainian language."
        )
        return self._query(system, user)

    def get_available_models(self) -> list[str]:
        try:
            url = f"{self.base_url}/api/tags"
            response = requests.get(url, timeout=5)
            if response.status_code == 200:
                models = [m["name"] for m in response.json().get("models", [])]
                if not models:
                    raise Exception("Ollama returned an empty list of models.")
                return models
            else:
                raise Exception(f"Ollama returned HTTP status {response.status_code}: {response.text}")
        except requests.exceptions.RequestException as re:
            raise Exception(f"Cannot connect to Ollama at {self.base_url}. Make sure Ollama is running.")
        except Exception as e:
            raise Exception(f"Ollama fetch models failed: {e}")


class LMStudioProvider(AIProvider):
    def __init__(self, config: dict):
        super().__init__(config)
        self.base_url = config.get("base_url", "http://localhost:1234/v1").rstrip("/")
        self.model = config.get("model", "meta-llama-3-8b-instruct")

    def test_connection(self) -> tuple[bool, str]:
        try:
            url = f"{self.base_url}/models"
            response = requests.get(url, timeout=5)
            if response.status_code == 200:
                models = [m["id"] for m in response.json().get("data", [])]
                return True, f"Connected to LM Studio! Loaded models: {', '.join(models)}"
            return False, f"LM Studio returned status code {response.status_code}"
        except requests.exceptions.ConnectionError:
            return False, "Cannot connect to LM Studio. Is it running?"
        except Exception as e:
            return False, f"LM Studio connection error: {str(e)}"

    def _query(self, system_prompt: str, user_prompt: str) -> str:
        try:
            url = f"{self.base_url}/chat/completions"
            payload = {
                "model": self.model,
                "messages": [
                    {"role": "system", "content": system_prompt},
                    {"role": "user", "content": user_prompt}
                ],
                "temperature": float(self.config.get("temperature", 0.7)),
                "stream": False
            }
            response = requests.post(url, json=payload, timeout=None)
            if response.status_code == 200:
                return response.json().get("choices", [{}])[0].get("message", {}).get("content", "Empty response")
            return f"LM Studio Error (HTTP {response.status_code}): {response.text}"
        except Exception as e:
            logger.error(f"LM Studio request error: {e}")
            return f"LM Studio request failed: {e}"

    def generate_recommendations(self, disk_summary: str, files_list: list) -> str:
        system = (
            "You are an SRE Storage Analytics assistant. Help the user optimize their disk space. "
            "You MUST write your response entirely in Ukrainian language. "
            "For each specific file path you recommend to delete, you MUST append a markdown link next to it in the exact format: "
            "[Видалити](delete://<absolute_path>). For example: "
            "'- C:\\Users\\Admin\\AppData\\Local\\Temp\\test.log (12.4 MB) - [Видалити](delete://C:/Users/Admin/AppData/Local/Temp/test.log)'."
        )
        user = f"Review the disk state and suggest items to clean up.\nDisk Status:\n{disk_summary}\n\nTop Large / Temp / Log files found:\n"
        for idx, f in enumerate(files_list[:15]):
            user += f"- {f['path']} ({f['size_formatted']}) - Category: {f['category']}\n"
        user += "\nProvide clear, structured markdown recommendations, risks of deleting, and an actionable cleanup plan. Write your response entirely in Ukrainian language."
        return self._query(system, user)

    def explain_folder(self, folder_path: str, details: dict) -> str:
        system = "You are an AI Storage Assistant explaining disk analysis results. You MUST write your response entirely in Ukrainian language."
        user = (
            f"Explain why this folder is marked for cleanup:\n"
            f"Folder: {folder_path}\n"
            f"Total Size: {details.get('size_formatted', 'unknown')}\n"
            f"File Types: {details.get('types', 'unknown')}\n"
            f"Last Accessed: {details.get('last_access', 'unknown')}\n"
            f"Risk Level: {details.get('risk_score', 'medium')}/100\n\n"
            f"Explain what this folder contains, what risks are associated with deleting it, "
            f"and what could be the potential savings. Write your response entirely in Ukrainian language."
        )
        return self._query(system, user)

    def get_available_models(self) -> list[str]:
        try:
            url = f"{self.base_url}/models"
            response = requests.get(url, timeout=5)
            if response.status_code == 200:
                models = [m["id"] for m in response.json().get("data", [])]
                if not models:
                    raise Exception("LM Studio returned an empty list of models.")
                return models
            else:
                raise Exception(f"LM Studio returned HTTP status {response.status_code}: {response.text}")
        except requests.exceptions.RequestException as re:
            raise Exception(f"Cannot connect to LM Studio at {self.base_url}. Make sure LM Studio is running.")
        except Exception as e:
            raise Exception(f"LM Studio fetch models failed: {e}")
