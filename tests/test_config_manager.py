import pytest
from src.config.config_manager import ConfigManager, RabbitMQConfig


class TestConfigManager:
    """
    Test class for ConfigManager functionality.
    """

    def test_config_manager_initialization_with_default_config(self):
        """
        Test ConfigManager initialization with default configuration.
        """
        config_manager = ConfigManager()
        
        assert config_manager.config_data is not None
        assert config_manager.config_data['rabbitmq_host'] == 'localhost'
        assert config_manager.config_data['rabbitmq_port'] == 5672

    def test_config_manager_initialization_with_custom_config(self):
        """
        Test ConfigManager initialization with custom configuration.
        """
        custom_config = {'rabbitmq_host': 'custom-host', 'rabbitmq_port': 5673}
        config_manager = ConfigManager(custom_config)
        
        assert config_manager.config_data['rabbitmq_host'] == 'custom-host'
        assert config_manager.config_data['rabbitmq_port'] == 5673

    def test_get_rabbitmq_config_returns_correct_object(self):
        """
        Test that get_rabbitmq_config returns a properly configured RabbitMQConfig object.
        """
        config_manager = ConfigManager()
        rabbitmq_config = config_manager.get_rabbitmq_config()
        
        assert isinstance(rabbitmq_config, RabbitMQConfig)
        assert rabbitmq_config.host == 'localhost'
        assert rabbitmq_config.port == 5672
        assert rabbitmq_config.virtual_host == 'guard_vhost'
        assert rabbitmq_config.username == 'user'
        assert rabbitmq_config.password == 'password'

    def test_get_config_value_with_existing_key(self):
        """
        Test getting a configuration value that exists.
        """
        config_manager = ConfigManager()
        host_value = config_manager.get_config_value('rabbitmq_host')
        
        assert host_value == 'localhost'

    def test_get_config_value_with_non_existing_key(self):
        """
        Test getting a configuration value that doesn't exist returns None.
        """
        config_manager = ConfigManager()
        non_existing_value = config_manager.get_config_value('non_existing_key')
        
        assert non_existing_value is None

    def test_get_config_value_with_default_value(self):
        """
        Test getting a configuration value with a default value.
        """
        config_manager = ConfigManager()
        default_value = config_manager.get_config_value('non_existing_key', 'default')
        
        assert default_value == 'default'

    def test_update_config_updates_value(self):
        """
        Test updating a configuration value.
        """
        config_manager = ConfigManager()
        config_manager.update_config('rabbitmq_host', 'new-host')
        
        assert config_manager.config_data['rabbitmq_host'] == 'new-host'

    def test_update_config_adds_new_key(self):
        """
        Test updating configuration with a new key.
        """
        config_manager = ConfigManager()
        config_manager.update_config('new_key', 'new_value')
        
        assert config_manager.config_data['new_key'] == 'new_value'


class TestRabbitMQConfig:
    """
    Test class for RabbitMQConfig dataclass.
    """

    def test_rabbitmq_config_initialization(self):
        """
        Test RabbitMQConfig initialization with all parameters.
        """
        config = RabbitMQConfig(
            host='localhost',
            port=5672,
            virtual_host='test_vhost',
            username='test_user',
            password='test_pass',
            exchange_name='test_exchange',
            routing_key='test.#',
            queue_name='test_queue'
        )
        
        assert config.host == 'localhost'
        assert config.port == 5672
        assert config.virtual_host == 'test_vhost'
        assert config.username == 'test_user'
        assert config.password == 'test_pass'
        assert config.exchange_name == 'test_exchange'
        assert config.routing_key == 'test.#'
        assert config.queue_name == 'test_queue'