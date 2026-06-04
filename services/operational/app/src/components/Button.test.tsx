import { fireEvent, render, screen } from "@testing-library/react-native";
import { ActivityIndicator } from "react-native";

import { Button } from "./Button";

describe("<Button>", () => {
  it("renders the label and fires onPress", () => {
    const onPress = jest.fn();
    render(<Button label="Sign in" onPress={onPress} />);

    fireEvent.press(screen.getByText("Sign in"));
    expect(onPress).toHaveBeenCalledTimes(1);
  });

  it("does not fire onPress while loading", () => {
    const onPress = jest.fn();
    render(<Button label="Sign in" onPress={onPress} loading />);

    // The label is replaced by ActivityIndicator while loading.
    expect(screen.queryByText("Sign in")).toBeNull();
    // Press through the spinner so the event traverses up to the (disabled)
    // Pressable and the disabled guard is honoured.
    fireEvent.press(screen.UNSAFE_getByType(ActivityIndicator));
    expect(onPress).not.toHaveBeenCalled();
  });

  it("does not fire onPress when disabled", () => {
    const onPress = jest.fn();
    render(<Button label="Disabled" onPress={onPress} disabled />);
    fireEvent.press(screen.getByText("Disabled"));
    expect(onPress).not.toHaveBeenCalled();
  });

  it("renders the secondary variant without crashing", () => {
    render(<Button label="Cancel" onPress={() => {}} variant="secondary" />);
    expect(screen.getByText("Cancel")).toBeTruthy();
  });
});
