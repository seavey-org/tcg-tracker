// Utility functions for card grid layout calculations
// Used across multiple screens: CollectionScreen, SearchScreen, DashboardScreen

/// Standard card aspect ratio (width/height) - Pokemon/MTG cards are 2.5" x 3.5"
const double cardAspectRatio = 2.5 / 3.5;

/// Calculate the number of columns based on screen width
int calculateColumns(double width) {
  if (width < 400) return 2;
  if (width < 600) return 3;
  if (width < 900) return 4;
  return 5;
}

/// Calculate aspect ratio for grid items based on screen width
/// Returns slightly taller cards on smaller screens for better visibility
double calculateGridAspectRatio(double width) {
  if (width < 400) return 0.55;
  if (width < 600) return 0.58;
  return 0.6;
}
