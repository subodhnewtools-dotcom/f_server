import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:freezed_annotation/freezed_annotation.dart';

part 'screen_stack_selection.freezed.dart';

@freezed
class StackSelection with _$StackSelection {
  const factory StackSelection({
    @Default(true) bool php,
    @Default(false) bool nodejs,
    @Default(false) bool redis,
    @Default(false) bool python,
  }) = _StackSelection;
}

class StackSelectionScreen extends ConsumerStatefulWidget {
  final Function(StackSelection) onComplete;
  final VoidCallback? onBack;

  const StackSelectionScreen({
    super.key,
    required this.onComplete,
    this.onBack,
  });

  @override
  ConsumerState<StackSelectionScreen> createState() =>
      _StackSelectionScreenState();
}

class _StackSelectionScreenState extends ConsumerState<StackSelectionScreen> {
  late StackSelection _selection;

  @override
  void initState() {
    super.initState();
    _selection = const StackSelection();
  }

  bool get _canProceed => _selection.php || _selection.nodejs;

  void _togglePhp(bool value) {
    setState(() {
      _selection = _selection.copyWith(php: value);
    });
  }

  void _toggleNodejs(bool value) {
    setState(() {
      _selection = _selection.copyWith(nodejs: value);
    });
  }

  void _toggleRedis(bool value) {
    setState(() {
      _selection = _selection.copyWith(redis: value);
    });
  }

  void _togglePython(bool value) {
    setState(() {
      _selection = _selection.copyWith(python: value);
    });
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Select Technology Stack'),
        leading: widget.onBack != null
            ? IconButton(
                icon: const Icon(Icons.arrow_back),
                onPressed: widget.onBack,
              )
            : null,
      ),
      body: Padding(
        padding: const EdgeInsets.all(24.0),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            const Text(
              'Which services do you want to enable?',
              style: TextStyle(
                fontSize: 24,
                fontWeight: FontWeight.bold,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              'You can always change this later in settings.',
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                    color: Colors.grey[600],
                  ),
            ),
            const SizedBox(height: 32),
            _buildStackCard(
              icon: Icons.code,
              title: 'PHP 8.3',
              subtitle: 'Run PHP applications with FPM',
              value: _selection.php,
              onChanged: _togglePhp,
              isRequired: true,
            ),
            const SizedBox(height: 16),
            _buildStackCard(
              icon: Icons.javascript,
              title: 'Node.js 20 LTS',
              subtitle: 'Run Node.js applications',
              value: _selection.nodejs,
              onChanged: _toggleNodejs,
              isRequired: false,
            ),
            const SizedBox(height: 16),
            _buildStackCard(
              icon: Icons.storage,
              title: 'Redis 7.2',
              subtitle: 'In-memory caching and sessions',
              value: _selection.redis,
              onChanged: _toggleRedis,
              isRequired: false,
            ),
            const SizedBox(height: 16),
            _buildStackCard(
              icon: Icons.python,
              title: 'Python (Coming Soon)',
              subtitle: 'Python application support',
              value: _selection.python,
              onChanged: _togglePython,
              isRequired: false,
              disabled: true,
            ),
            const Spacer(),
            FilledButton(
              onPressed: _canProceed ? () => widget.onComplete(_selection) : null,
              child: const Padding(
                padding: EdgeInsets.all(16.0),
                child: Text('Continue'),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildStackCard({
    required IconData icon,
    required String title,
    required String subtitle,
    required bool value,
    required Function(bool) onChanged,
    required bool isRequired,
    bool disabled = false,
  }) {
    return Card(
      elevation: value ? 2 : 0,
      color: disabled
          ? Colors.grey[200]
          : value
              ? Theme.of(context).colorScheme.primaryContainer
              : null,
      child: CheckboxListTile(
        value: value,
        onChanged: disabled ? null : onChanged,
        title: Row(
          children: [
            Icon(icon, color: disabled ? Colors.grey : null),
            const SizedBox(width: 12),
            Expanded(
              child: Text(
                title,
                style: TextStyle(
                  fontWeight: FontWeight.w600,
                  decoration: disabled ? TextDecoration.lineThrough : null,
                ),
              ),
            ),
            if (isRequired)
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
                decoration: BoxDecoration(
                  color: Colors.amber[100],
                  borderRadius: BorderRadius.circular(4),
                ),
                child: const Text(
                  'Required',
                  style: TextStyle(fontSize: 12, color: Colors.amber[900]),
                ),
              ),
          ],
        ),
        subtitle: Text(subtitle),
        controlAffinity: ListTileControlAffinity.leading,
      ),
    );
  }
}
