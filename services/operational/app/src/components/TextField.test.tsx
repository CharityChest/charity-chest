import { fireEvent, render, screen } from "@testing-library/react-native";

import { TextField } from "./TextField";

describe("<TextField>", () => {
  it("renders the label above the input", () => {
    render(<TextField label="Email" value="" onChangeText={() => {}} placeholder="you@x" />);
    expect(screen.getByText("Email")).toBeTruthy();
    expect(screen.getByPlaceholderText("you@x")).toBeTruthy();
  });

  it("emits onChangeText", () => {
    const onChangeText = jest.fn();
    render(
      <TextField label="Email" value="" onChangeText={onChangeText} placeholder="you@x" />
    );
    fireEvent.changeText(screen.getByPlaceholderText("you@x"), "alice@example.com");
    expect(onChangeText).toHaveBeenCalledWith("alice@example.com");
  });

  it("defaults autoCapitalize to 'none' (avoids iOS title-casing emails)", () => {
    render(<TextField label="Email" value="" onChangeText={() => {}} placeholder="you@x" />);
    const input = screen.getByPlaceholderText("you@x");
    expect(input.props.autoCapitalize).toBe("none");
  });

  it("respects a caller-supplied autoCapitalize", () => {
    render(
      <TextField
        label="Name"
        value=""
        onChangeText={() => {}}
        placeholder="Your name"
        autoCapitalize="words"
      />
    );
    expect(screen.getByPlaceholderText("Your name").props.autoCapitalize).toBe("words");
  });

  it("links the label to the input for screen readers", () => {
    render(<TextField label="Email" value="" onChangeText={() => {}} placeholder="you@x" />);
    const labelId = screen.getByText("Email").props.nativeID;
    expect(labelId).toBeTruthy();
    expect(screen.getByPlaceholderText("you@x").props.accessibilityLabelledBy).toBe(labelId);
  });

  it("lets callers override the accessibility association", () => {
    render(
      <TextField
        label="Email"
        value=""
        onChangeText={() => {}}
        placeholder="you@x"
        accessibilityLabel="Email address"
      />
    );
    expect(screen.getByPlaceholderText("you@x").props.accessibilityLabel).toBe("Email address");
  });
});
