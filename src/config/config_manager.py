from dataclasses import dataclass
from typing import Dict, Any


@dataclass
class RabbitMQConfig:
    """
    Configuration class for RabbitMQ connection parameters.
    """
    
    host: str
    port: int
    virtual_host: str
    username: str
    password: str
    exchange_name: str
    routing_key: str
    queue_name: str


class ConfigManager:
    """
    A class to manage application configuration settings.
    """

    def __init__(self, config_data: Dict[str, Any] = None) -> None:
        """
        Initializes the configuration manager with provided data.

        Args:
            config_data (Dict[str, Any], optional): Configuration data dictionary.
        """
        self.config_data = config_data or self._get_default_config()

    def _get_default_config(self) -> Dict[str, Any]:
        """
        Returns the default configuration values.

        Returns:
            Dict[str, Any]: Default configuration dictionary.
        """
        return {
            'rabbitmq_host': 'localhost',
            'rabbitmq_port': 5672,
            'rabbitmq_vhost': 'guard_vhost',
            'rabbitmq_user': 'user',
            'rabbitmq_pass': 'password',
            'exchange_name': 'carnes_nobres',
            'routing_key': 'camera.#',
            'queue_name': 'test_consumer_queue'
        }

    def get_rabbitmq_config(self) -> RabbitMQConfig:
        """
        Creates and returns a RabbitMQ configuration object.

        Returns:
            RabbitMQConfig: Configured RabbitMQ parameters.
        """
        return RabbitMQConfig(
            host=self.config_data.get('rabbitmq_host'),
            port=self.config_data.get('rabbitmq_port'),
            virtual_host=self.config_data.get('rabbitmq_vhost'),
            username=self.config_data.get('rabbitmq_user'),
            password=self.config_data.get('rabbitmq_pass'),
            exchange_name=self.config_data.get('exchange_name'),
            routing_key=self.config_data.get('routing_key'),
            queue_name=self.config_data.get('queue_name')
        )

    def get_config_value(self, key: str, default_value: Any = None) -> Any:
        """
        Retrieves a configuration value by key.

        Args:
            key (str): The configuration key to retrieve.
            default_value (Any, optional): Default value if key not found.

        Returns:
            Any: The configuration value.
        """
        return self.config_data.get(key, default_value)

    def update_config(self, key: str, value: Any) -> None:
        """
        Updates a configuration value.

        Args:
            key (str): The configuration key to update.
            value (Any): The new value to set.
        """
        self.config_data[key] = value