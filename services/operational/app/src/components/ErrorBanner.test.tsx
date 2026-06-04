import { render, screen } from "@testing-library/react-native";

import { ErrorBanner } from "./ErrorBanner";

describe("<ErrorBanner>", () => {
  it("renders the message when one is provided", () => {
    render(<ErrorBanner message="Something broke" />);
    expect(screen.getByText("Something broke")).toBeTruthy();
  });

  it("renders nothing when message is null", () => {
    render(<ErrorBanner message={null} />);
    expect(screen.queryByText(/./)).toBeNull();
  });

  it("renders nothing when message is an empty string", () => {
    // Empty string is falsy; the component bails out the same way as null.
    render(<ErrorBanner message="" />);
    expect(screen.queryByText(/./)).toBeNull();
  });
});
