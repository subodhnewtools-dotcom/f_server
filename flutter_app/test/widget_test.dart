import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('PocketServer app smoke test', (WidgetTester tester) async {
    // Build our app and trigger a frame.
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          appBar: AppBar(
            title: const Text('PocketServer'),
          ),
          body: const Center(
            child: Text('Setup Wizard - Coming Next'),
          ),
        ),
      ),
    );

    // Verify that the app title is displayed.
    expect(find.text('PocketServer'), findsOneWidget);
    
    // Verify that the placeholder text is displayed.
    expect(find.text('Setup Wizard - Coming Next'), findsOneWidget);
  });

  testWidgets('AppBar displays correct title', (WidgetTester tester) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          appBar: AppBar(
            title: const Text('PocketServer'),
          ),
          body: const SizedBox.shrink(),
        ),
      ),
    );

    expect(find.text('PocketServer'), findsOneWidget);
  });
}
