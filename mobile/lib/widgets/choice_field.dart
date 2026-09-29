import 'package:flutter/material.dart';

class ChoiceOption {
  const ChoiceOption({required this.id, required this.label});

  final String id;
  final String label;
}

class ChoiceField extends StatelessWidget {
  const ChoiceField({
    super.key,
    required this.label,
    required this.value,
    required this.enabled,
    required this.onTap,
  });

  final String label;
  final String? value;
  final bool enabled;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Padding(
      padding: const EdgeInsets.only(bottom: 10),
      child: InkWell(
        onTap: enabled ? onTap : null,
        borderRadius: BorderRadius.circular(12),
        child: InputDecorator(
          decoration: InputDecoration(
            labelText: label,
            enabled: enabled,
            suffixIcon: const Icon(Icons.unfold_more),
          ),
          child: Text(
            value ?? (enabled ? 'Bitte wählen' : '—'),
            style: TextStyle(
              color: value == null ? scheme.onSurfaceVariant : scheme.onSurface,
            ),
          ),
        ),
      ),
    );
  }
}

Future<String?> showChoiceSheet(
  BuildContext context, {
  required String title,
  required List<ChoiceOption> options,
  String? selectedId,
}) {
  return showModalBottomSheet<String>(
    context: context,
    showDragHandle: true,
    builder: (context) {
      final height = MediaQuery.sizeOf(context).height * 0.55;
      return SafeArea(
        child: SizedBox(
          height: height,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Padding(
                padding: const EdgeInsets.fromLTRB(20, 0, 20, 8),
                child: Text(
                  title,
                  style: Theme.of(context).textTheme.titleMedium,
                ),
              ),
              Expanded(
                child: ListView.builder(
                  itemCount: options.length,
                  itemBuilder: (context, index) {
                    final option = options[index];
                    final selected = option.id == selectedId;
                    return ListTile(
                      title: Text(option.label),
                      trailing: selected ? const Icon(Icons.check) : null,
                      onTap: () => Navigator.pop(context, option.id),
                    );
                  },
                ),
              ),
            ],
          ),
        ),
      );
    },
  );
}
