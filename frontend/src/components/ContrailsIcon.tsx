import { CSSProperties } from "react";
import logo from '../assets/images/logo.png'

interface ContrailsIconProps {
  className?: string;
  style?: CSSProperties;
}

export function ContrailsIcon({
  className,
  style,
}: ContrailsIconProps) {
  return (
    <img src={logo} alt="Contrails" className={`icon-invert-logo${className ? ` ${className}` : ''}`} style={{ width: '34px', ...style }} />
  );
}
