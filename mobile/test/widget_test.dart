import 'package:flutter_test/flutter_test.dart';
import 'package:mobile/main.dart';

void main() {
  testWidgets('App loads without error', (WidgetTester tester) async {
    await tester.pumpWidget(const TCGTrackerApp());
    expect(find.text('Scan Card'), findsOneWidget);
  });
}
