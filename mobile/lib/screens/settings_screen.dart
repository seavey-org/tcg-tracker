import 'package:flutter/material.dart';
import '../services/api_service.dart';

class SettingsScreen extends StatefulWidget {
  final ApiService? apiService;

  const SettingsScreen({super.key, this.apiService});

  @override
  State<SettingsScreen> createState() => _SettingsScreenState();
}

class _SettingsScreenState extends State<SettingsScreen> {
  late final ApiService _apiService;
  final TextEditingController _serverUrlController = TextEditingController();
  final _formKey = GlobalKey<FormState>();
  bool _isLoading = true;
  bool _isTesting = false;
  bool? _connectionSuccess;

  @override
  void initState() {
    super.initState();
    _apiService = widget.apiService ?? ApiService();
    _loadSettings();
  }

  Future<void> _loadSettings() async {
    final serverUrl = await _apiService.getServerUrl();
    _serverUrlController.text = serverUrl;
    setState(() => _isLoading = false);
  }

  String? _validateUrl(String? value) {
    if (value == null || value.trim().isEmpty) {
      return 'Server URL cannot be empty';
    }

    final url = value.trim();
    try {
      final uri = Uri.parse(url);
      if (!uri.hasScheme || (uri.scheme != 'http' && uri.scheme != 'https')) {
        return 'URL must start with http:// or https://';
      }
      if (uri.host.isEmpty) {
        return 'Please enter a valid server address';
      }
    } catch (e) {
      return 'Please enter a valid URL';
    }

    return null;
  }

  Future<void> _testConnection() async {
    final url = _serverUrlController.text.trim();
    final validationError = _validateUrl(url);
    if (validationError != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(validationError),
          backgroundColor: Theme.of(context).colorScheme.error,
        ),
      );
      return;
    }

    setState(() {
      _isTesting = true;
      _connectionSuccess = null;
    });

    // Temporarily set the URL to test
    final originalUrl = await _apiService.getServerUrl();
    await _apiService.setServerUrl(url);

    try {
      final success = await _apiService.testConnection();
      if (mounted) {
        setState(() {
          _connectionSuccess = success;
          _isTesting = false;
        });

        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(success
                ? 'Connection successful!'
                : 'Could not connect to server'),
            backgroundColor:
                success ? Colors.green : Theme.of(context).colorScheme.error,
            behavior: SnackBarBehavior.floating,
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _connectionSuccess = false;
          _isTesting = false;
        });
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Connection failed: $e'),
            backgroundColor: Theme.of(context).colorScheme.error,
            behavior: SnackBarBehavior.floating,
          ),
        );
      }
    }

    // Restore original URL if test failed
    if (_connectionSuccess != true) {
      await _apiService.setServerUrl(originalUrl);
    }
  }

  Future<void> _saveSettings() async {
    if (!_formKey.currentState!.validate()) {
      return;
    }

    final url = _serverUrlController.text.trim();
    await _apiService.setServerUrl(url);

    if (!mounted) return;

    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(
        content: Text('Settings saved!'),
        backgroundColor: Colors.green,
        behavior: SnackBarBehavior.floating,
      ),
    );

    // Clear connection status after saving
    setState(() => _connectionSuccess = null);
  }

  @override
  void dispose() {
    _serverUrlController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    return Scaffold(
      appBar: AppBar(
        title: const Text('Settings'),
      ),
      body: _isLoading
          ? const Center(child: CircularProgressIndicator())
          : Form(
              key: _formKey,
              child: ListView(
                padding: const EdgeInsets.all(16),
                children: [
                  Text(
                    'Server Configuration',
                    style: theme.textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                  const SizedBox(height: 8),
                  Text(
                    'Enter the URL of your TCG Tracker backend server.',
                    style: theme.textTheme.bodyMedium?.copyWith(
                      color: colorScheme.onSurfaceVariant,
                    ),
                  ),
                  const SizedBox(height: 16),
                  TextFormField(
                    controller: _serverUrlController,
                    decoration: InputDecoration(
                      labelText: 'Server URL',
                      hintText: 'http://192.168.1.100:8080',
                      border: const OutlineInputBorder(),
                      prefixIcon: const Icon(Icons.link),
                      suffixIcon: _connectionSuccess != null
                          ? Icon(
                              _connectionSuccess!
                                  ? Icons.check_circle
                                  : Icons.error,
                              color: _connectionSuccess!
                                  ? Colors.green
                                  : colorScheme.error,
                            )
                          : null,
                    ),
                    keyboardType: TextInputType.url,
                    validator: _validateUrl,
                    onChanged: (_) {
                      // Clear connection status when URL changes
                      if (_connectionSuccess != null) {
                        setState(() => _connectionSuccess = null);
                      }
                    },
                  ),
                  const SizedBox(height: 16),
                  Row(
                    children: [
                      Expanded(
                        child: OutlinedButton.icon(
                          onPressed: _isTesting ? null : _testConnection,
                          icon: _isTesting
                              ? const SizedBox(
                                  width: 18,
                                  height: 18,
                                  child: CircularProgressIndicator(strokeWidth: 2),
                                )
                              : const Icon(Icons.wifi_find),
                          label: Text(_isTesting ? 'Testing...' : 'Test Connection'),
                        ),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: FilledButton.icon(
                          onPressed: _saveSettings,
                          icon: const Icon(Icons.save),
                          label: const Text('Save'),
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 32),
                  const Divider(),
                  const SizedBox(height: 16),
                  Card(
                    child: ListTile(
                      leading: Icon(
                        Icons.info_outline,
                        color: colorScheme.primary,
                      ),
                      title: const Text('TCG Tracker Mobile'),
                      subtitle: const Text('Version 1.0.0'),
                    ),
                  ),
                  const SizedBox(height: 8),
                  Card(
                    child: ListTile(
                      leading: Icon(
                        Icons.help_outline,
                        color: colorScheme.primary,
                      ),
                      title: const Text('Connection Tips'),
                      subtitle: const Text(
                        'Make sure your phone and server are on the same network. '
                        'Use your computer\'s local IP address (e.g., 192.168.x.x), not localhost.',
                      ),
                    ),
                  ),
                ],
              ),
            ),
    );
  }
}
