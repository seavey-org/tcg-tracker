import 'package:flutter/material.dart';
import '../services/api_service.dart';

class SettingsScreen extends StatefulWidget {
  const SettingsScreen({super.key});

  @override
  State<SettingsScreen> createState() => _SettingsScreenState();
}

class _SettingsScreenState extends State<SettingsScreen> {
  final ApiService _apiService = ApiService();
  final TextEditingController _serverUrlController = TextEditingController();
  bool _isLoading = true;

  @override
  void initState() {
    super.initState();
    _loadSettings();
  }

  Future<void> _loadSettings() async {
    final serverUrl = await _apiService.getServerUrl();
    _serverUrlController.text = serverUrl;
    setState(() => _isLoading = false);
  }

  Future<void> _saveSettings() async {
    final url = _serverUrlController.text.trim();
    if (url.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Server URL cannot be empty')),
      );
      return;
    }

    await _apiService.setServerUrl(url);

    if (!mounted) return;

    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(
        content: Text('Settings saved!'),
        backgroundColor: Colors.green,
      ),
    );
  }

  @override
  void dispose() {
    _serverUrlController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Settings'),
        backgroundColor: Theme.of(context).colorScheme.inversePrimary,
      ),
      body: _isLoading
          ? const Center(child: CircularProgressIndicator())
          : ListView(
              padding: const EdgeInsets.all(16),
              children: [
                const Text(
                  'Server Configuration',
                  style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
                ),
                const SizedBox(height: 8),
                const Text(
                  'Enter the URL of your TCG Tracker backend server.',
                  style: TextStyle(color: Colors.grey),
                ),
                const SizedBox(height: 16),
                TextField(
                  controller: _serverUrlController,
                  decoration: const InputDecoration(
                    labelText: 'Server URL',
                    hintText: 'http://192.168.1.100:8080',
                    border: OutlineInputBorder(),
                    prefixIcon: Icon(Icons.link),
                  ),
                  keyboardType: TextInputType.url,
                ),
                const SizedBox(height: 24),
                FilledButton(
                  onPressed: _saveSettings,
                  child: const Text('Save Settings'),
                ),
                const SizedBox(height: 32),
                const Divider(),
                const SizedBox(height: 16),
                const ListTile(
                  leading: Icon(Icons.info),
                  title: Text('TCG Tracker Mobile'),
                  subtitle: Text('Version 1.0.0'),
                ),
              ],
            ),
    );
  }
}
