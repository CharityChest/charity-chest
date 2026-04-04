import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import ErrorBanner from './ErrorBanner';

describe('ErrorBanner', () => {
  it('renders nothing when message is empty', () => {
    const { container } = render(<ErrorBanner message="" />);
    expect(container).toBeEmptyDOMElement();
  });

  it('renders the message text', () => {
    render(<ErrorBanner message="credenziali non valide" />);
    expect(screen.getByText('credenziali non valide')).toBeInTheDocument();
  });

  it('has role="alert" for screen reader accessibility', () => {
    render(<ErrorBanner message="something went wrong" />);
    expect(screen.getByRole('alert')).toBeInTheDocument();
  });

  it('renders the warning icon (aria-hidden SVG)', () => {
    const { container } = render(<ErrorBanner message="error" />);
    const svg = container.querySelector('svg[aria-hidden="true"]');
    expect(svg).not.toBeNull();
  });

  it('applies the left-accent border styling', () => {
    render(<ErrorBanner message="error" />);
    const alert = screen.getByRole('alert');
    expect(alert.className).toContain('border-l-4');
    expect(alert.className).toContain('border-red-500');
  });
});
