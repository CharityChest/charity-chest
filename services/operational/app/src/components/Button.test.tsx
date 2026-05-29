import { fireEvent, render, screen } from "@testing-library/react-native";

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
    // Still press the underlying pressable by walking up from the spinner.
    const pressable = screen.UNSAFE_getByType(
      // eslint-disable-next-line @typescript-eslint/no-var-requires
      require("react-native").Pressable
    );
    fireEvent.press(pressable);
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
