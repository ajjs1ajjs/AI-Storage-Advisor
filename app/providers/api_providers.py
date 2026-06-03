import requests
from app.providers.base import AIProvider
from app.core.config import logger

class OpenAIAPIProvider(AIProvider):
    def __init__(self, config: dict):
        super().__init__(config)
        self.api_key = config.get("api_key", "")
        self.model = config.get("model", "gpt-5.4-mini")
        try:
            self.temp = float(config.get("temperature", 0.7))
        except (ValueError, TypeError):
            self.temp = 0.7

    def test_connection(self) -> tuple[bool, str]:
        if not self.api_key:
            return False, "API key is missing."
        try:
            url = "https://api.openai.com/v1/chat/completions"
            headers = {
                "Authorization": f"Bearer {self.api_key}",
                "Content-Type": "application/json"
            }
            payload = {
                "model": self.model,
                "messages": [{"role": "user", "content": "Ping"}],
                "max_tokens": 5
            }
            res = requests.post(url, json=payload, headers=headers, timeout=10)
            if res.status_code == 200:
                return True, "OpenAI API connection successful!"
            return False, f"OpenAI API Error {res.status_code}: {res.json().get('error', {}).get('message', res.text)}"
        except Exception as e:
            return False, f"OpenAI connection failed: {e}"

    def _query(self, system: str, user: str) -> str:
        try:
            url = "https://api.openai.com/v1/chat/completions"
            headers = {
                "Authorization": f"Bearer {self.api_key}",
                "Content-Type": "application/json"
            }
            payload = {
                "model": self.model,
                "messages": [
                    {"role": "system", "content": system},
                    {"role": "user", "content": user}
                ],
                "temperature": self.temp
            }
            res = requests.post(url, json=payload, headers=headers, timeout=60)
            if res.status_code == 200:
                return res.json()["choices"][0]["message"]["content"]
            return f"OpenAI Error: {res.text}"
        except Exception as e:
            return f"OpenAI Request failed: {e}"

    def generate_recommendations(self, disk_summary: str, files_list: list) -> str:
        system = self.get_recommendation_system_prompt()
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
        if not self.api_key:
            raise Exception("OpenAI API Key is missing. Please enter it first.")
        try:
            url = "https://api.openai.com/v1/models"
            headers = {"Authorization": f"Bearer {self.api_key}"}
            res = requests.get(url, headers=headers, timeout=10)
            if res.status_code == 200:
                models = [m["id"] for m in res.json().get("data", [])]
                gpt_models = [m for m in models if "gpt" in m or "o1" in m]
                return sorted(gpt_models) if gpt_models else sorted(models)
            else:
                try:
                    err_msg = res.json().get("error", {}).get("message", res.text)
                except Exception:
                    err_msg = res.text
                raise Exception(f"OpenAI API Error {res.status_code}: {err_msg}")
        except requests.exceptions.RequestException as re:
            raise Exception(f"OpenAI connection error: {re}")
        except Exception as e:
            raise Exception(f"OpenAI fetch models failed: {e}")


class AnthropicAPIProvider(AIProvider):
    def __init__(self, config: dict):
        super().__init__(config)
        self.api_key = config.get("api_key", "")
        self.model = config.get("model", "claude-sonnet-4.6")
        try:
            self.temp = float(config.get("temperature", 0.7))
        except (ValueError, TypeError):
            self.temp = 0.7

    def test_connection(self) -> tuple[bool, str]:
        if not self.api_key:
            return False, "API key is missing."
        try:
            url = "https://api.anthropic.com/v1/messages"
            headers = {
                "x-api-key": self.api_key,
                "anthropic-version": "2023-06-01",
                "Content-Type": "application/json"
            }
            payload = {
                "model": self.model,
                "messages": [{"role": "user", "content": "Ping"}],
                "max_tokens": 5
            }
            res = requests.post(url, json=payload, headers=headers, timeout=10)
            if res.status_code == 200:
                return True, "Anthropic API connection successful!"
            return False, f"Anthropic API Error {res.status_code}: {res.json().get('error', {}).get('message', res.text)}"
        except Exception as e:
            return False, f"Anthropic connection failed: {e}"

    def _query(self, system: str, user: str) -> str:
        try:
            url = "https://api.anthropic.com/v1/messages"
            headers = {
                "x-api-key": self.api_key,
                "anthropic-version": "2023-06-01",
                "Content-Type": "application/json"
            }
            payload = {
                "model": self.model,
                "system": system,
                "messages": [{"role": "user", "content": user}],
                "max_tokens": 4096,
                "temperature": self.temp
            }
            res = requests.post(url, json=payload, headers=headers, timeout=60)
            if res.status_code == 200:
                return res.json()["content"][0]["text"]
            return f"Anthropic Error: {res.text}"
        except Exception as e:
            return f"Anthropic Request failed: {e}"

    def generate_recommendations(self, disk_summary: str, files_list: list) -> str:
        system = self.get_recommendation_system_prompt()
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
        if not self.api_key:
            raise Exception("Anthropic API Key is missing. Please enter it first.")
        try:
            url = "https://api.anthropic.com/v1/models"
            headers = {
                "x-api-key": self.api_key,
                "anthropic-version": "2023-06-01"
            }
            res = requests.get(url, headers=headers, timeout=10)
            if res.status_code == 200:
                models = sorted([m["id"] for m in res.json().get("data", [])])
                if not models:
                    raise Exception("Anthropic returned an empty list of models.")
                return models
            else:
                try:
                    err_msg = res.json().get("error", {}).get("message", res.text)
                except Exception:
                    err_msg = res.text
                raise Exception(f"Anthropic API Error {res.status_code}: {err_msg}")
        except requests.exceptions.RequestException as re:
            raise Exception(f"Anthropic connection error: {re}")
        except Exception as e:
            raise Exception(f"Anthropic fetch models failed: {e}")


class GeminiAPIProvider(AIProvider):
    def __init__(self, config: dict):
        super().__init__(config)
        self.api_key = config.get("api_key", "")
        self.model = config.get("model", "gemini-3.5-flash")
        try:
            self.temp = float(config.get("temperature", 0.7))
        except (ValueError, TypeError):
            self.temp = 0.7

    def test_connection(self) -> tuple[bool, str]:
        if not self.api_key:
            return False, "API key is missing."
        try:
            url = f"https://generativelanguage.googleapis.com/v1beta/models/{self.model}:generateContent?key={self.api_key}"
            headers = {"Content-Type": "application/json"}
            payload = {
                "contents": [{"parts": [{"text": "Ping"}]}]
            }
            res = requests.post(url, json=payload, headers=headers, timeout=10)
            if res.status_code == 200:
                return True, "Gemini API connection successful!"
            return False, f"Gemini API Error {res.status_code}: {res.text}"
        except Exception as e:
            return False, f"Gemini connection failed: {e}"

    def _query(self, system: str, user: str) -> str:
        try:
            url = f"https://generativelanguage.googleapis.com/v1beta/models/{self.model}:generateContent?key={self.api_key}"
            headers = {"Content-Type": "application/json"}
            
            # Since Gemini beta supports systemInstruction
            payload = {
                "contents": [{"parts": [{"text": user}]}],
                "systemInstruction": {"parts": [{"text": system}]},
                "generationConfig": {"temperature": self.temp}
            }
            res = requests.post(url, json=payload, headers=headers, timeout=60)
            if res.status_code == 200:
                return res.json()["candidates"][0]["content"]["parts"][0]["text"]
            return f"Gemini Error: {res.text}"
        except Exception as e:
            return f"Gemini Request failed: {e}"

    def generate_recommendations(self, disk_summary: str, files_list: list) -> str:
        system = self.get_recommendation_system_prompt()
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
        if not self.api_key:
            raise Exception("Gemini API Key is missing. Please enter it first.")
        try:
            url = f"https://generativelanguage.googleapis.com/v1beta/models?key={self.api_key}"
            res = requests.get(url, timeout=10)
            if res.status_code == 200:
                models = []
                for m in res.json().get("models", []):
                    name = m.get("name", "")
                    if name.startswith("models/"):
                        models.append(name.replace("models/", ""))
                if not models:
                    raise Exception("Gemini returned an empty list of models.")
                return sorted(models)
            else:
                try:
                    err_msg = res.json().get("error", {}).get("message", res.text)
                except Exception:
                    err_msg = res.text
                raise Exception(f"Gemini API Error {res.status_code}: {err_msg}")
        except requests.exceptions.RequestException as re:
            raise Exception(f"Gemini connection error: {re}")
        except Exception as e:
            raise Exception(f"Gemini fetch models failed: {e}")


class DeepSeekAPIProvider(AIProvider):
    def __init__(self, config: dict):
        super().__init__(config)
        self.api_key = config.get("api_key", "")
        self.model = config.get("model", "deepseek-v4-flash")
        try:
            self.temp = float(config.get("temperature", 0.7))
        except (ValueError, TypeError):
            self.temp = 0.7

    def test_connection(self) -> tuple[bool, str]:
        if not self.api_key:
            return False, "API key is missing."
        try:
            url = "https://api.deepseek.com/chat/completions"
            headers = {
                "Authorization": f"Bearer {self.api_key}",
                "Content-Type": "application/json"
            }
            payload = {
                "model": self.model,
                "messages": [{"role": "user", "content": "Ping"}],
                "max_tokens": 5
            }
            res = requests.post(url, json=payload, headers=headers, timeout=10)
            if res.status_code == 200:
                return True, "DeepSeek API connection successful!"
            return False, f"DeepSeek API Error {res.status_code}: {res.json().get('error', {}).get('message', res.text)}"
        except Exception as e:
            return False, f"DeepSeek connection failed: {e}"

    def _query(self, system: str, user: str) -> str:
        try:
            url = "https://api.deepseek.com/chat/completions"
            headers = {
                "Authorization": f"Bearer {self.api_key}",
                "Content-Type": "application/json"
            }
            payload = {
                "model": self.model,
                "messages": [
                    {"role": "system", "content": system},
                    {"role": "user", "content": user}
                ],
                "temperature": self.temp
            }
            res = requests.post(url, json=payload, headers=headers, timeout=60)
            if res.status_code == 200:
                return res.json()["choices"][0]["message"]["content"]
            return f"DeepSeek Error: {res.text}"
        except Exception as e:
            return f"DeepSeek Request failed: {e}"

    def generate_recommendations(self, disk_summary: str, files_list: list) -> str:
        system = self.get_recommendation_system_prompt()
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
        if not self.api_key:
            raise Exception("DeepSeek API Key is missing. Please enter it first.")
        try:
            url = "https://api.deepseek.com/models"
            headers = {"Authorization": f"Bearer {self.api_key}"}
            res = requests.get(url, headers=headers, timeout=10)
            if res.status_code == 200:
                models = sorted([m["id"] for m in res.json().get("data", [])])
                if not models:
                    raise Exception("DeepSeek returned an empty list of models.")
                return models
            else:
                try:
                    err_msg = res.json().get("error", {}).get("message", res.text)
                except Exception:
                    err_msg = res.text
                raise Exception(f"DeepSeek API Error {res.status_code}: {err_msg}")
        except requests.exceptions.RequestException as re:
            raise Exception(f"DeepSeek connection error: {re}")
        except Exception as e:
            raise Exception(f"DeepSeek fetch models failed: {e}")


class CustomAPIProvider(AIProvider):
    def __init__(self, config: dict):
        super().__init__(config)
        self.base_url = config.get("base_url", "https://api.openai.com/v1").rstrip("/")
        self.api_key = config.get("api_key", "")
        self.model = config.get("model", "gpt-4")
        try:
            self.temp = float(config.get("temperature", 0.7))
        except (ValueError, TypeError):
            self.temp = 0.7

    def test_connection(self) -> tuple[bool, str]:
        try:
            url = f"{self.base_url}/chat/completions"
            headers = {
                "Content-Type": "application/json"
            }
            if self.api_key:
                headers["Authorization"] = f"Bearer {self.api_key}"
            payload = {
                "model": self.model,
                "messages": [{"role": "user", "content": "Ping"}],
                "max_tokens": 5
            }
            res = requests.post(url, json=payload, headers=headers, timeout=10)
            if res.status_code == 200:
                return True, "Custom API connection successful!"
            return False, f"Custom API Error {res.status_code}: {res.text}"
        except Exception as e:
            return False, f"Custom API connection failed: {e}"

    def _query(self, system: str, user: str) -> str:
        try:
            url = f"{self.base_url}/chat/completions"
            headers = {
                "Content-Type": "application/json"
            }
            if self.api_key:
                headers["Authorization"] = f"Bearer {self.api_key}"
            payload = {
                "model": self.model,
                "messages": [
                    {"role": "system", "content": system},
                    {"role": "user", "content": user}
                ],
                "temperature": self.temp
            }
            res = requests.post(url, json=payload, headers=headers, timeout=180)
            if res.status_code == 200:
                return res.json()["choices"][0]["message"]["content"]
            return f"Custom API Error: {res.text}"
        except Exception as e:
            return f"Custom API Request failed: {e}"

    def generate_recommendations(self, disk_summary: str, files_list: list) -> str:
        system = self.get_recommendation_system_prompt()
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
            headers = {}
            if self.api_key:
                headers["Authorization"] = f"Bearer {self.api_key}"
            res = requests.get(url, headers=headers, timeout=10)
            if res.status_code == 200:
                models = sorted([m["id"] for m in res.json().get("data", [])])
                if not models:
                    raise Exception("Custom API returned an empty list of models.")
                return models
            else:
                try:
                    err_msg = res.json().get("error", {}).get("message", res.text)
                except Exception:
                    err_msg = res.text
                raise Exception(f"Custom API Error {res.status_code}: {err_msg}")
        except requests.exceptions.RequestException as re:
            raise Exception(f"Custom API connection error: {re}")
        except Exception as e:
            raise Exception(f"Custom API fetch models failed: {e}")
